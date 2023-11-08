/*
Copyright 2023 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package authorizer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	authzconfig "k8s.io/apiserver/pkg/apis/apiserver"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/authorization/authorizerfactory"
	"k8s.io/apiserver/pkg/authorization/union"
	"k8s.io/apiserver/pkg/server/options/authorizationconfig/metrics"
	webhookutil "k8s.io/apiserver/pkg/util/webhook"
	"k8s.io/apiserver/plugin/pkg/authorizer/webhook"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/auth/authorizer/abac"
	"k8s.io/kubernetes/pkg/kubeapiserver/authorizer/modes"
	"k8s.io/kubernetes/plugin/pkg/auth/authorizer/node"
	"k8s.io/kubernetes/plugin/pkg/auth/authorizer/rbac"
)

type reloadableAuthorizerResolver struct {
	initialConfig Config

	apiServerID string

	reloadInterval         time.Duration
	requireNonWebhookTypes sets.Set[authzconfig.AuthorizerType]

	nodeAuthorizer *node.NodeAuthorizer
	rbacAuthorizer *rbac.RBACAuthorizer
	abacAuthorizer abac.PolicyList

	lastLoadedLock   sync.Mutex
	lastLoadedConfig *authzconfig.AuthorizationConfiguration
	lastReadData     []byte

	current atomic.Pointer[authorizerResolver]
}

type authorizerResolver struct {
	authorizer   authorizer.Authorizer
	ruleResolver authorizer.RuleResolver
}

func (r *reloadableAuthorizerResolver) Authorize(ctx context.Context, a authorizer.Attributes) (authorized authorizer.Decision, reason string, err error) {
	return r.current.Load().authorizer.Authorize(ctx, a)
}

func (r *reloadableAuthorizerResolver) RulesFor(user user.Info, namespace string) ([]authorizer.ResourceRuleInfo, []authorizer.NonResourceRuleInfo, bool, error) {
	return r.current.Load().ruleResolver.RulesFor(user, namespace)
}

// newForConfig constructs
func (r *reloadableAuthorizerResolver) newForConfig(authzConfig *authzconfig.AuthorizationConfiguration) (authorizer.Authorizer, authorizer.RuleResolver, error) {
	if len(authzConfig.Authorizers) == 0 {
		return nil, nil, fmt.Errorf("at least one authorization mode must be passed")
	}

	var (
		authorizers   []authorizer.Authorizer
		ruleResolvers []authorizer.RuleResolver
	)

	// Add SystemPrivilegedGroup as an authorizing group
	superuserAuthorizer := authorizerfactory.NewPrivilegedGroups(user.SystemPrivilegedGroup)
	authorizers = append(authorizers, superuserAuthorizer)

	for _, configuredAuthorizer := range authzConfig.Authorizers {
		// Keep cases in sync with constant list in k8s.io/kubernetes/pkg/kubeapiserver/authorizer/modes/modes.go.
		switch configuredAuthorizer.Type {
		case authzconfig.AuthorizerType(modes.ModeNode):
			if r.nodeAuthorizer == nil {
				return nil, nil, fmt.Errorf("nil nodeAuthorizer")
			}
			authorizers = append(authorizers, r.nodeAuthorizer)
			ruleResolvers = append(ruleResolvers, r.nodeAuthorizer)
		case authzconfig.AuthorizerType(modes.ModeAlwaysAllow):
			alwaysAllowAuthorizer := authorizerfactory.NewAlwaysAllowAuthorizer()
			authorizers = append(authorizers, alwaysAllowAuthorizer)
			ruleResolvers = append(ruleResolvers, alwaysAllowAuthorizer)
		case authzconfig.AuthorizerType(modes.ModeAlwaysDeny):
			alwaysDenyAuthorizer := authorizerfactory.NewAlwaysDenyAuthorizer()
			authorizers = append(authorizers, alwaysDenyAuthorizer)
			ruleResolvers = append(ruleResolvers, alwaysDenyAuthorizer)
		case authzconfig.AuthorizerType(modes.ModeABAC):
			if r.abacAuthorizer == nil {
				return nil, nil, fmt.Errorf("nil abacAuthorizer")
			}
			authorizers = append(authorizers, r.abacAuthorizer)
			ruleResolvers = append(ruleResolvers, r.abacAuthorizer)
		case authzconfig.AuthorizerType(modes.ModeWebhook):
			if r.initialConfig.WebhookRetryBackoff == nil {
				return nil, nil, errors.New("retry backoff parameters for authorization webhook has not been specified")
			}
			clientConfig, err := webhookutil.LoadKubeconfig(*configuredAuthorizer.Webhook.ConnectionInfo.KubeConfigFile, r.initialConfig.CustomDial)
			if err != nil {
				return nil, nil, err
			}
			var decisionOnError authorizer.Decision
			switch configuredAuthorizer.Webhook.FailurePolicy {
			case authzconfig.FailurePolicyNoOpinion:
				decisionOnError = authorizer.DecisionNoOpinion
			case authzconfig.FailurePolicyDeny:
				decisionOnError = authorizer.DecisionDeny
			default:
				return nil, nil, fmt.Errorf("unknown failurePolicy %q", configuredAuthorizer.Webhook.FailurePolicy)
			}
			webhookAuthorizer, err := webhook.New(clientConfig,
				configuredAuthorizer.Webhook.SubjectAccessReviewVersion,
				configuredAuthorizer.Webhook.AuthorizedTTL.Duration,
				configuredAuthorizer.Webhook.UnauthorizedTTL.Duration,
				*r.initialConfig.WebhookRetryBackoff,
				decisionOnError,
				configuredAuthorizer.Webhook.MatchConditions,
			)
			if err != nil {
				return nil, nil, err
			}
			authorizers = append(authorizers, webhookAuthorizer)
			ruleResolvers = append(ruleResolvers, webhookAuthorizer)
		case authzconfig.AuthorizerType(modes.ModeRBAC):
			if r.rbacAuthorizer == nil {
				return nil, nil, fmt.Errorf("nil rbacAuthorizer")
			}
			authorizers = append(authorizers, r.rbacAuthorizer)
			ruleResolvers = append(ruleResolvers, r.rbacAuthorizer)
		default:
			return nil, nil, fmt.Errorf("unknown authorization mode %s specified", configuredAuthorizer.Type)
		}
	}

	return union.New(authorizers...), union.NewRuleResolvers(ruleResolvers...), nil
}

// runReload starts checking the config file for changes and reloads the authorizer when it changes.
// Blocks until ctx is complete.
func (r *reloadableAuthorizerResolver) runReload(ctx context.Context) {
	metrics.RegisterMetrics()
	metrics.RecordAuthorizationConfigAutomaticReloadSuccess(r.apiServerID)

	go func() {
		if err := r.watchFile(ctx); err != nil {
			select {
			case <-ctx.Done():
				// server was shutting down, we don't care about reload errors
			default:
				klog.ErrorS(err, "watching authorization config file")
			}
		}
	}()

	_ = wait.PollUntilContextCancel(ctx, r.reloadInterval, false, func(context.Context) (exitPoll bool, exitPollErr error) {
		r.checkFile(ctx)
		return false, nil
	})
}

// watchFile sets up a file watch. Blocks until a file watch error is encountered or the context is complete.
func (r *reloadableAuthorizerResolver) watchFile(ctx context.Context) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("error creating fsnotify watcher: %v", err)
	}
	defer func() {
		_ = w.Close()
	}()

	if err = w.Add(r.initialConfig.ReloadFile); err != nil {
		return fmt.Errorf("adding watch for file %s: %w", r.initialConfig.ReloadFile, err)
	}
	// Trigger a check in case the file was updated before the watch started
	r.checkFile(ctx)

	for {
		select {
		case e := <-w.Events:
			select {
			case <-ctx.Done():
				// server is done
				return nil
			default:
				if err := r.handleConfigFileEvent(ctx, e, w); err != nil {
					return err
				}
			}
		case err := <-w.Errors:
			return fmt.Errorf("received fsnotify error: %v", err)
		case <-ctx.Done():
			// server is done
			return nil
		}
	}
}

func (r *reloadableAuthorizerResolver) handleConfigFileEvent(ctx context.Context, e fsnotify.Event, w *fsnotify.Watcher) error {
	// This should be executed after restarting the watch (if applicable) to ensure no file event will be missing.
	defer r.checkFile(ctx)
	if !e.Has(fsnotify.Remove) && !e.Has(fsnotify.Rename) {
		return nil
	}
	_ = w.Remove(r.initialConfig.ReloadFile)
	if err := w.Add(r.initialConfig.ReloadFile); err != nil {
		return fmt.Errorf("error adding watch for file %s: %v", r.initialConfig.ReloadFile, err)
	}
	return nil
}

func (r *reloadableAuthorizerResolver) checkFile(ctx context.Context) {
	r.lastLoadedLock.Lock()
	defer r.lastLoadedLock.Unlock()

	data, err := os.ReadFile(r.initialConfig.ReloadFile)
	if err != nil {
		klog.ErrorS(err, "reloading authorization config")
		metrics.RecordAuthorizationConfigAutomaticReloadFailure(r.apiServerID)
		return
	}
	if bytes.Equal(data, r.lastReadData) {
		// no change
		return
	}
	klog.InfoS("found new authorization config data")
	r.lastReadData = data

	config, err := LoadAndValidateData(data, r.requireNonWebhookTypes)
	if err != nil {
		klog.ErrorS(err, "reloading authorization config")
		metrics.RecordAuthorizationConfigAutomaticReloadFailure(r.apiServerID)
		return
	}
	if reflect.DeepEqual(config, r.lastLoadedConfig) {
		// no change
		return
	}
	klog.InfoS("found new authorization config")
	r.lastLoadedConfig = config

	authorizer, ruleResolver, err := r.newForConfig(config)
	if err != nil {
		klog.ErrorS(err, "reloading authorization config")
		metrics.RecordAuthorizationConfigAutomaticReloadFailure(r.apiServerID)
		return
	}
	klog.InfoS("constructed new authorizer")

	r.current.Store(&authorizerResolver{
		authorizer:   authorizer,
		ruleResolver: ruleResolver,
	})
	klog.InfoS("reloaded authz config")
	metrics.RecordAuthorizationConfigAutomaticReloadSuccess(r.apiServerID)
}

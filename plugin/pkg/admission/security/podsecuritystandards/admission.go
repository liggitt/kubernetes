/*
Copyright 2021 The Kubernetes Authors.

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

package podsecuritystandards

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	genericadmissioninit "k8s.io/apiserver/pkg/admission/initializer"
	"k8s.io/apiserver/pkg/audit"
	"k8s.io/apiserver/pkg/warning"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"
	"k8s.io/kubernetes/pkg/apis/core"
	internalcorev1 "k8s.io/kubernetes/pkg/apis/core/v1"
)

// PluginName is a string with the name of the plugin
const PluginName = "PodSecurityStandards"

// Register registers a plugin
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(config io.Reader) (admission.Interface, error) {
		plugin := newPlugin()
		return plugin, nil
	})
}

// Plugin holds state for and implements the admission plugin.
type Plugin struct {
	*admission.Handler

	defaultAllowLevel   string
	defaultAllowVersion Version

	ignoreUsers      sets.String
	ignoreNamespaces sets.String

	client          kubernetes.Interface
	namespaceLister corev1listers.NamespaceLister
	podLister       corev1listers.PodLister
}

// ValidateInitialization ensures an authorizer is set.
func (p *Plugin) ValidateInitialization() error {
	if p.namespaceLister == nil {
		return fmt.Errorf("%s requires a lister", PluginName)
	}
	if p.podLister == nil {
		return fmt.Errorf("%s requires a lister", PluginName)
	}
	if p.client == nil {
		return fmt.Errorf("%s requires a client", PluginName)
	}
	return nil
}

var _ admission.ValidationInterface = &Plugin{}
var _ genericadmissioninit.WantsExternalKubeInformerFactory = &Plugin{}
var _ genericadmissioninit.WantsExternalKubeClientSet = &Plugin{}

// newPlugin creates a new admission plugin.
func newPlugin() *Plugin {
	return &Plugin{
		Handler: admission.NewHandler(admission.Create, admission.Update),

		// TODO: allow configuring these
		defaultAllowLevel:   LevelPrivileged,
		defaultAllowVersion: Version{latest: true},

		ignoreUsers:      sets.NewString(),
		ignoreNamespaces: sets.NewString(),
	}
}

// SetExternalKubeInformerFactory registers an informer
func (p *Plugin) SetExternalKubeInformerFactory(f informers.SharedInformerFactory) {
	namespaceInformer := f.Core().V1().Namespaces()
	p.namespaceLister = namespaceInformer.Lister()
	p.podLister = f.Core().V1().Pods().Lister()
	p.SetReadyFunc(namespaceInformer.Informer().HasSynced)
}

// SetExternalKubeClientSet sets the plugin's client
func (p *Plugin) SetExternalKubeClientSet(client kubernetes.Interface) {
	p.client = client
}

var (
	pods       = corev1.Resource("pods")
	namespaces = corev1.Resource("namespaces")
)

// Validate verifies attributes against the PodSecurityPolicy
func (p *Plugin) Validate(ctx context.Context, a admission.Attributes, o admission.ObjectInterfaces) error {
	if p.ignoreUsers.Has(a.GetUserInfo().GetName()) {
		return nil
	}
	if p.ignoreNamespaces.Has(a.GetNamespace()) {
		return nil
	}

	switch a.GetResource().GroupResource() {
	case pods:
		return p.validatePod(ctx, a)
	case namespaces:
		return p.validateNamespace(ctx, a)
	default:
		return nil
	}
}

func (p *Plugin) validatePod(ctx context.Context, a admission.Attributes) error {
	podSpec := &corev1.PodSpec{}

	switch a.GetSubresource() {
	case "":
		pod, ok := a.GetObject().(*core.Pod)
		if !ok {
			return fmt.Errorf("expected pod, got %T", a.GetObject())
		}
		if err := internalcorev1.Convert_core_PodSpec_To_v1_PodSpec(&pod.Spec, podSpec, nil); err != nil {
			return err
		}

	case "ephemeralcontainers":
		// TODO: ephemeral containers

	default:
		// TODO: audit/warn on workload types containing PodSpec?
		// CronJob, Job, Deployment, ReplicaSet, ReplicationController, StatefulSet, DaemonSet, PodTemplate?
		return nil
	}

	// get the namespace
	namespace, err := p.namespaceLister.Get(a.GetNamespace())
	if apierrors.IsNotFound(err) {
		// do a live lookup if not found
		namespace, err = p.client.CoreV1().Namespaces().Get(ctx, a.GetNamespace(), metav1.GetOptions{})
	}
	if err != nil {
		return admission.NewForbidden(a, err)
	}

	// TODO: cache evaluations

	// TODO: fallback to cluster-wide audit level if unspecified
	if level, ok := namespace.Labels[AuditLabel]; ok {
		evaluateLevel, _ := LevelToEvaluate(level)
		evaluateVersion, _ := VersionToEvaluate(namespace.Labels[AuditVersionLabel])
		errs := policies[evaluateLevel](podSpec, evaluateVersion)

		audit.AddAuditAnnotation(ctx, AuditLabel, evaluateLevel)
		if evaluateVersion.latest {
			audit.AddAuditAnnotation(ctx, AuditVersionLabel, "latest")
		} else {
			audit.AddAuditAnnotation(ctx, AuditVersionLabel, namespace.Labels[AuditVersionLabel])
		}

		if len(errs) == 0 {
			audit.AddAuditAnnotation(ctx, AuditLabel+".decision", "allow")
		} else {
			audit.AddAuditAnnotation(ctx, AuditLabel+".decision", "forbid")
			audit.AddAuditAnnotation(ctx, AuditLabel+".error", strings.Join(errs, "; "))
		}
	}

	// TODO: fallback to cluster-wide warn level if unspecified
	if level, ok := namespace.Labels[WarnLabel]; ok {
		evaluateLevel, _ := LevelToEvaluate(level)
		evaluateVersion, _ := VersionToEvaluate(namespace.Labels[WarnVersionLabel])
		errs := policies[evaluateLevel](podSpec, evaluateVersion)
		if len(errs) > 0 {
			warning.AddWarning(ctx, "", fmt.Sprintf("would be disallowed in namespace %q by %q policy: %s", namespace.Name, evaluateLevel, strings.Join(errs, "; ")))
		}
	}

	// TODO: fallback to cluster-wide enforce level if unspecified
	if level, ok := namespace.Labels[AllowLabel]; ok {
		// TODO: skip enforcement on no-op updates
		evaluateLevel, _ := LevelToEvaluate(level)
		evaluateVersion, _ := VersionToEvaluate(namespace.Labels[AllowVersionLabel])
		errs := policies[evaluateLevel](podSpec, evaluateVersion)

		audit.AddAuditAnnotation(ctx, AllowLabel, evaluateLevel)
		if evaluateVersion.latest {
			audit.AddAuditAnnotation(ctx, AllowLabel, "latest")
		} else {
			audit.AddAuditAnnotation(ctx, AllowVersionLabel, namespace.Labels[AllowVersionLabel])
		}

		if len(errs) == 0 {
			audit.AddAuditAnnotation(ctx, AllowLabel+".decision", "allow")
		} else {
			disallowed := strings.Join(errs, "; ")
			audit.AddAuditAnnotation(ctx, AllowLabel+".decision", "forbid")
			audit.AddAuditAnnotation(ctx, AllowLabel+".error", disallowed)
			return admission.NewForbidden(a, fmt.Errorf("disallowed in namespace %q by %q policy: %s", namespace.Name, evaluateLevel, disallowed))
		}
	}

	return nil
}

func (p *Plugin) validateNamespace(ctx context.Context, a admission.Attributes) error {
	namespace, ok := a.GetObject().(*core.Namespace)
	if !ok {
		return fmt.Errorf("expected namespace, got %T", a.GetObject())
	}

	newLabels := namespace.Labels
	var oldLabels map[string]string
	if oldObject := a.GetOldObject(); oldObject != nil {
		if oldNamespace, ok := oldObject.(*core.Namespace); !ok {
			return fmt.Errorf("expected old object to be namespace, got %T", oldObject)
		} else {
			oldLabels = oldNamespace.Labels
		}
	}

	var errs []error
	for _, levelLabelKey := range levelKeys {
		newValue, hasLabel := newLabels[levelLabelKey]
		if !hasLabel {
			// no-op, incoming object does not have the label
			continue
		}
		oldValue, hadLabel := oldLabels[levelLabelKey]
		if newValue == oldValue && hasLabel == hadLabel {
			// no-op, API call is not changing presence or value
			continue
		}

		if _, err := LevelToEvaluate(newValue); err != nil {
			errs = append(errs, fmt.Errorf("invalid %q level %q: %v", levelLabelKey, newValue, err))
		}
	}

	for levelLabelKey, versionLabelKey := range levelToVersionKeys {
		newValue, hasLabel := newLabels[versionLabelKey]
		if !hasLabel {
			// no-op, incoming object does not have the label
			continue
		}

		_, hadLevelLabel := oldLabels[levelLabelKey]
		_, hasLevelLabel := newLabels[levelLabelKey]
		if hadLevelLabel && !hasLevelLabel {
			// trying to remove the level label but leave the version label
			errs = append(errs, fmt.Errorf("cannot set %q label without setting %q label", versionLabelKey, levelLabelKey))
			continue
		}

		oldValue, hadLabel := oldLabels[versionLabelKey]
		if newValue == oldValue && hasLabel == hadLabel {
			// no-op, API call is not changing presence or value
			continue
		}

		if _, err := VersionToEvaluate(newValue); err != nil {
			errs = append(errs, fmt.Errorf("invalid %q version %q: %v", versionLabelKey, newValue, err))
		}
	}

	if len(errs) == 0 && !p.ignoreNamespaces.Has(namespace.Name) {
		p.warnOnEnforceLevelChange(ctx, namespace.Name, newLabels, oldLabels)
	}

	return nil
}

func (p *Plugin) warnOnEnforceLevelChange(ctx context.Context, namespace string, newLabels, oldLabels map[string]string) {
	newAllowLevel := p.defaultAllowLevel
	newAllowVersion := p.defaultAllowVersion
	if _, hasExplicitAllowLevel := newLabels[AllowLabel]; hasExplicitAllowLevel {
		newAllowLevel, _ = LevelToEvaluate(newLabels[AllowLabel])
		newAllowVersion, _ = VersionToEvaluate(newLabels[AllowVersionLabel])
	}

	oldAllowLevel := p.defaultAllowLevel
	oldAllowVersion := p.defaultAllowVersion
	if _, hasExplicitAllowLevel := oldLabels[AllowLabel]; hasExplicitAllowLevel {
		oldAllowLevel, _ = LevelToEvaluate(oldLabels[AllowLabel])
		oldAllowVersion, _ = VersionToEvaluate(oldLabels[AllowVersionLabel])
	}

	if newAllowLevel == oldAllowLevel && newAllowVersion == oldAllowVersion {
		// no change in effective allow level/version
		return
	}

	if newAllowLevel == LevelPrivileged {
		// now privileged, all shall pass
		return
	}

	// check existing pods and warn if they would not be permitted by the new policy
	pods, err := p.podLister.Pods(namespace).List(labels.Everything())
	if err != nil {
		warning.AddWarning(ctx, "", "could not look up existing pods to check compatibility with new allow policy")
		return
	}

	// TODO: be smarter about only surfacing identical warnings once for pods created from the same controller

	contextTimeout := ctx.Done()
	localTimer := time.NewTimer(5 * time.Second)
	defer localTimer.Stop()
	for _, pod := range pods {
		select {
		case <-contextTimeout:
			// request timed out
			return
		case <-localTimer.C:
			// limit on pod evaluation timed out
			warning.AddWarning(ctx, "", "timed out checking compatibility of existing pods with new allow policy")
			return
		default:
			podErrs := policies[newAllowLevel](&pod.Spec, newAllowVersion)
			if len(podErrs) > 0 {
				warning.AddWarning(ctx, "", fmt.Sprintf("pod %q in namespace %q would be disallowed by %q policy: %s", pod.Name, namespace, newAllowLevel, strings.Join(podErrs, "; ")))
			}
		}
	}
}

type validateFunc func(spec *corev1.PodSpec, v Version) []string

var policies = map[string]validateFunc{
	LevelPrivileged: func(spec *corev1.PodSpec, v Version) []string {
		return nil
	},
	LevelBaseline: func(spec *corev1.PodSpec, v Version) []string {
		var errs []string
		errs = append(errs, evaluate(spec, v, baselineChecks)...)
		return errs
	},
	LevelRestricted: func(spec *corev1.PodSpec, v Version) []string {
		var errs []string
		errs = append(errs, evaluate(spec, v, baselineChecks)...)
		errs = append(errs, evaluate(spec, v, restrictedChecks)...)
		return errs
	},
}

func evaluate(spec *corev1.PodSpec, v Version, checks []check) []string {
	var errs []string
	for _, check := range checks {
		if check.min.GT(v) || check.max.LT(v) {
			continue
		}
		if check.checkPod != nil {
			if !check.checkPod(spec) {
				errs = append(errs, check.disallowed)
				continue
			}
		}
		if check.checkContainer != nil {
			sawBadContainer := false
			podutil.VisitContainers(spec, podutil.AllContainers, func(container *corev1.Container, containerType podutil.ContainerType) bool {
				if !check.checkContainer(container) {
					sawBadContainer = true
				}
				return !sawBadContainer
			})
			if sawBadContainer {
				errs = append(errs, check.disallowed)
				continue
			}
		}
	}
	return errs
}

type check struct {
	min            Version
	max            Version
	disallowed     string
	checkPod       func(*corev1.PodSpec) bool
	checkContainer func(*corev1.Container) bool
}

// do not change the contents of this set.
// if new capabilities are allowed in the future, create a new set at that version.
// TODO: case-sensitive? prefixed with CAP_?
var v0DefaultCapabilities = sets.NewString(
	"CAP_AUDIT_WRITE",
	"CAP_CHOWN",
	"CAP_DAC_OVERRIDE",
	"CAP_FOWNER",
	"CAP_FSETID",
	"CAP_KILL",
	"CAP_MKNOD",
	"CAP_NET_BIND_SERVICE",
	"CAP_NET_RAW",
	"CAP_SETFCAP",
	"CAP_SETGID",
	"CAP_SETPCAP",
	"CAP_SETUID",
	"CAP_SYS_CHROOT",
)

// do not change the contents of this set.
// if new sysctls are allowed in the future, create a new set at that version.
var v0AllowedSysctls = sets.NewString(
	"kernel.shm_rmid_forced",
	"net.ipv4.ip_local_port_range",
	"net.ipv4.tcp_syncookies",
	"net.ipv4.ping_group_range",
)

var baselineChecks = []check{
	{
		disallowed: "hostNetwork=true",
		checkPod: func(spec *corev1.PodSpec) bool {
			return spec.HostNetwork == false
		},
	},
	{
		disallowed: "hostPID=true",
		checkPod: func(spec *corev1.PodSpec) bool {
			return spec.HostPID == false
		},
	},
	{
		disallowed: "hostIPC=true",
		checkPod: func(spec *corev1.PodSpec) bool {
			return spec.HostIPC == false
		},
	},
	// Privileged Pods disable most security mechanisms and must be disallowed.
	{
		disallowed: "privileged containers",
		checkContainer: func(container *corev1.Container) bool {
			return container.SecurityContext == nil || container.SecurityContext.Privileged == nil || *container.SecurityContext.Privileged == false
		},
	},
	// Adding additional capabilities beyond the default set must be disallowed.
	{
		disallowed: "adding non-default capabilities",
		checkContainer: func(container *corev1.Container) bool {
			if container.SecurityContext != nil && container.SecurityContext.Capabilities != nil {
				for _, cap := range container.SecurityContext.Capabilities.Add {
					if !v0DefaultCapabilities.Has(string(cap)) {
						return false
					}
				}
			}
			return true
		},
	},
	// HostPorts should be disallowed, or at minimum restricted to a known list.
	{
		disallowed: "host ports",
		checkContainer: func(container *corev1.Container) bool {
			for _, port := range container.Ports {
				if port.HostPort != 0 {
					return false
				}
			}
			return true
		},
	},
	// The default /proc masks are set up to reduce attack surface, and should be required.
	{
		disallowed: "procMount != Default",
		checkContainer: func(container *corev1.Container) bool {
			return container.SecurityContext == nil || container.SecurityContext.ProcMount == nil || *container.SecurityContext.ProcMount == corev1.DefaultProcMount
		},
	},
	// HostPath volumes must be forbidden.
	{
		disallowed: "hostPath volumes",
		checkPod: func(spec *corev1.PodSpec) bool {
			for _, volume := range spec.Volumes {
				if volume.VolumeSource.HostPath != nil {
					return false
				}
			}
			return true
		},
	},
	// Sysctls can disable security mechanisms or affect all containers on a host, and should be disallowed except for an allowed "safe" subset.
	// A sysctl is considered safe if it is namespaced in the container or the Pod, and it is isolated from other Pods or processes on the same Node.
	{
		disallowed: "unsafe sysctls",
		checkPod: func(spec *corev1.PodSpec) bool {
			if spec.SecurityContext != nil {
				for _, sysctl := range spec.SecurityContext.Sysctls {
					if !v0AllowedSysctls.Has(sysctl.Name) {
						return false
					}
				}
			}
			return true
		},
	},

	// TODO? On supported hosts, the 'runtime/default' AppArmor profile is applied by default. The default policy should prevent overriding or disabling the policy, or restrict overrides to an allowed set of profiles.

	// TODO? Setting custom SELinux options should be disallowed.
}

var emptyVolumeSource = corev1.VolumeSource{}

var restrictedChecks = []check{
	// In addition to restricting HostPath volumes, the restricted profile limits usage of non-core volume types to those defined through PersistentVolumes.
	{
		disallowed: "volumes other than secret, configmap, emptyDir, downwardAPI, projected, persistentVolumeClaim, and ephemeral volumes",
		checkPod: func(spec *corev1.PodSpec) bool {
			for _, volume := range spec.Volumes {
				// hostPath is already enforced by default policy
				volume.HostPath = nil
				// nil out other allowed volumes
				volume.Secret = nil
				volume.ConfigMap = nil
				volume.EmptyDir = nil
				volume.DownwardAPI = nil
				volume.Projected = nil
				volume.PersistentVolumeClaim = nil
				volume.Ephemeral = nil
				// make sure no other volume source is set
				if !reflect.DeepEqual(volume.VolumeSource, emptyVolumeSource) {
					return false
				}
			}
			return true
		},
	},
	// 1.8+: Privilege escalation (such as via set-user-ID or set-group-ID file mode) should not be allowed.
	{
		min:        Version{minor: 8},
		disallowed: "allowPrivilegeEscalation != false",
		checkContainer: func(container *corev1.Container) bool {
			return container.SecurityContext != nil && container.SecurityContext.AllowPrivilegeEscalation != nil && *container.SecurityContext.AllowPrivilegeEscalation == false
		},
	},
	// Containers must be required to run as non-root users.
	{
		disallowed: "runAsNonRoot != true",
		checkPod: func(spec *corev1.PodSpec) bool {
			var podRunAsNonRoot *bool
			if spec.SecurityContext != nil {
				podRunAsNonRoot = spec.SecurityContext.RunAsNonRoot
			}
			if podRunAsNonRoot != nil && *podRunAsNonRoot == false {
				// the pod explicitly set a bad setting
				return false
			}

			sawBadContainer := false
			podutil.VisitContainers(spec, podutil.AllContainers, func(container *corev1.Container, containerType podutil.ContainerType) bool {
				var containerRunAsNonRoot *bool
				if container.SecurityContext != nil {
					containerRunAsNonRoot = container.SecurityContext.RunAsNonRoot
				}
				switch {
				case containerRunAsNonRoot != nil && *containerRunAsNonRoot == false:
					// the container explicitly set a bad setting
					sawBadContainer = true
				case containerRunAsNonRoot == nil && podRunAsNonRoot == nil:
					// the container didn't set a good setting and the pod didn't set a good setting
					sawBadContainer = true
				}
				return !sawBadContainer
			})
			return sawBadContainer
		},
	},
	// TODO: Containers should be forbidden from running with a root primary or supplementary GID.

	// TODO: The RuntimeDefault seccomp profile must be required, or allow specific additional profiles.
}

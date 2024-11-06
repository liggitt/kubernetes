/*
Copyright 2024 The Kubernetes Authors.

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

package plugin

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	externaljwtv1alpha1 "k8s.io/externaljwt/apis/v1alpha1"
	"k8s.io/kubernetes/pkg/serviceaccount"
)

func TestExternalPublicKeyGetter(t *testing.T) {
	testCases := []struct {
		desc                 string
		expectedErr          error
		supportedKeys        map[string]supportedKeyT
		wantVerificationKeys *VerificationKeys
		refreshHintSec       int
	}{
		{
			desc: "single key in signer",
			supportedKeys: map[string]supportedKeyT{
				"key-1": {
					key: &rsaKey1.PublicKey,
				},
			},
			wantVerificationKeys: &VerificationKeys{
				Keys: []serviceaccount.PublicKey{
					{
						KeyID:                    "key-1",
						PublicKey:                &rsaKey1.PublicKey,
						ExcludeFromOIDCDiscovery: false,
					},
				},
			},
			refreshHintSec: 20,
		},
		{
			desc: "multiple keys in signer",
			supportedKeys: map[string]supportedKeyT{
				"key-1": {
					key: &rsaKey1.PublicKey,
				},
				"key-2": {
					key:             &rsaKey2.PublicKey,
					excludeFromOidc: true,
				},
			},
			wantVerificationKeys: &VerificationKeys{
				Keys: []serviceaccount.PublicKey{
					{
						KeyID:                    "key-1",
						PublicKey:                &rsaKey1.PublicKey,
						ExcludeFromOIDCDiscovery: false,
					},
					{
						KeyID:                    "key-2",
						PublicKey:                &rsaKey2.PublicKey,
						ExcludeFromOIDCDiscovery: true,
					},
				},
			},
			refreshHintSec: 10,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			ctx := context.Background()

			sockname := fmt.Sprintf("@test-external-public-key-getter-%d.sock", i)
			t.Cleanup(func() { _ = os.Remove(sockname) })

			addr := &net.UnixAddr{Name: sockname, Net: "unix"}
			listener, err := net.ListenUnix(addr.Network(), addr)
			if err != nil {
				t.Fatalf("Failed to start fake backend: %v", err)
			}

			grpcServer := grpc.NewServer()

			backend := &dummyExtrnalSigner{
				supportedKeys:      tc.supportedKeys,
				refreshHintSeconds: tc.refreshHintSec,
			}
			externaljwtv1alpha1.RegisterExternalJWTSignerServer(grpcServer, backend)

			defer grpcServer.Stop()
			go func() {
				if err := grpcServer.Serve(listener); err != nil {
					panic(fmt.Errorf("error returned from grpcServer: %w", err))
				}
			}()

			clientConn, err := grpc.DialContext(
				ctx,
				sockname,
				grpc.WithContextDialer(func(ctx context.Context, path string) (net.Conn, error) {
					return (&net.Dialer{}).DialContext(ctx, "unix", path)
				}),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			if err != nil {
				t.Fatalf("Failed to dial buffconn client: %v", err)
			}
			defer func() {
				_ = clientConn.Close()
			}()

			plugin := newPlugin("iss", clientConn, true)

			signingKeys, err := plugin.keyCache.getTokenVerificationKeys(ctx)
			if err != nil {
				if tc.expectedErr == nil {
					t.Fatalf("error getting supported keys: %v", err)
				}
				if tc.expectedErr.Error() != err.Error() {
					t.Fatalf("want error: %v, got error: %v", tc.expectedErr, err)
					return
				}
			}

			if tc.expectedErr == nil {
				if diff := cmp.Diff(signingKeys.Keys, tc.wantVerificationKeys.Keys, cmpopts.SortSlices(sortPublicKeySlice)); diff != "" {
					t.Fatalf("Bad result from GetTokenSigningKeys; diff (-got +want)\n%s", diff)
				}
				expectedRefreshHintSec := time.Now().Add(time.Duration(tc.refreshHintSec) * time.Second)
				difference := signingKeys.NextRefreshHint.Sub(expectedRefreshHintSec).Seconds()
				if difference > 1 || difference < -1 { // tolerate 1 sec of skew for test
					t.Fatalf("refreshHint not as expected; got: %v want: %v", signingKeys.NextRefreshHint, expectedRefreshHintSec)
				}
			}
		})
	}
}

func TestInitialFill(t *testing.T) {
	ctx := context.Background()

	sockname := "@test-initial-fill.sock"
	t.Cleanup(func() { _ = os.Remove(sockname) })

	addr := &net.UnixAddr{Name: sockname, Net: "unix"}
	listener, err := net.ListenUnix(addr.Network(), addr)
	if err != nil {
		t.Fatalf("Failed to start fake backend: %v", err)
	}

	grpcServer := grpc.NewServer()

	supportedKeys := map[string]supportedKeyT{
		"key-1": {
			key: &rsaKey1.PublicKey,
		},
	}
	wantPubKeys := []serviceaccount.PublicKey{
		{
			KeyID:                    "key-1",
			PublicKey:                &rsaKey1.PublicKey,
			ExcludeFromOIDCDiscovery: false,
		},
	}

	backend := &dummyExtrnalSigner{
		supportedKeys:      supportedKeys,
		refreshHintSeconds: 10,
	}
	externaljwtv1alpha1.RegisterExternalJWTSignerServer(grpcServer, backend)

	defer grpcServer.Stop()
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			panic(fmt.Errorf("error returned from grpcServer: %w", err))
		}
	}()

	clientConn, err := grpc.DialContext(
		ctx,
		sockname,
		grpc.WithContextDialer(func(ctx context.Context, path string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", path)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to dial buffconn client: %v", err)
	}
	defer func() { _ = clientConn.Close() }()

	plugin := newPlugin("iss", clientConn, true)

	if err := plugin.keyCache.initialFill(ctx); err != nil {
		t.Fatalf("Error during InitialFill: %v", err)
	}

	gotPubKeys := plugin.keyCache.GetPublicKeys(ctx, "")
	if diff := cmp.Diff(gotPubKeys, wantPubKeys); diff != "" {
		t.Fatalf("Bad public keys; diff (-got +want)\n%s", diff)
	}
}

func TestReflectChanges(t *testing.T) {
	ctx := context.Background()

	sockname := "@test-reflect-changes.sock"
	t.Cleanup(func() { _ = os.Remove(sockname) })

	addr := &net.UnixAddr{Name: sockname, Net: "unix"}
	listener, err := net.ListenUnix(addr.Network(), addr)
	if err != nil {
		t.Fatalf("Failed to start fake backend: %v", err)
	}

	grpcServer := grpc.NewServer()

	supportedKeysT1 := map[string]supportedKeyT{
		"key-1": {
			key: &rsaKey1.PublicKey,
		},
	}
	wantPubKeysT1 := []serviceaccount.PublicKey{
		{
			KeyID:                    "key-1",
			PublicKey:                &rsaKey1.PublicKey,
			ExcludeFromOIDCDiscovery: false,
		},
	}

	backend := &dummyExtrnalSigner{
		supportedKeys:      supportedKeysT1,
		refreshHintSeconds: 10,
	}
	externaljwtv1alpha1.RegisterExternalJWTSignerServer(grpcServer, backend)

	defer grpcServer.Stop()
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			panic(fmt.Errorf("error returned from grpcServer: %w", err))
		}
	}()

	clientConn, err := grpc.DialContext(
		ctx,
		sockname,
		grpc.WithContextDialer(func(ctx context.Context, path string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", path)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to dial buffconn client: %v", err)
	}
	defer func() { _ = clientConn.Close() }()

	plugin := newPlugin("iss", clientConn, true)

	if err := plugin.keyCache.initialFill(ctx); err != nil {
		t.Fatalf("Error during InitialFill: %v", err)
	}

	gotPubKeysT1 := plugin.keyCache.GetPublicKeys(ctx, "")
	if diff := cmp.Diff(gotPubKeysT1, wantPubKeysT1, cmpopts.SortSlices(sortPublicKeySlice)); diff != "" {
		t.Fatalf("Bad public keys; diff (-got +want)\n%s", diff)
	}

	if _, err := plugin.keyCache.syncKeys(ctx); err != nil {
		t.Fatalf("Error while calling syncKeys: %v", err)
	}

	supportedKeysT2 := map[string]supportedKeyT{
		"key-1": {
			key:             &rsaKey1.PublicKey,
			excludeFromOidc: true,
		},
		"key-2": {
			key: &rsaKey2.PublicKey,
		},
	}
	wantPubKeysT2 := []serviceaccount.PublicKey{
		{
			KeyID:                    "key-1",
			PublicKey:                &rsaKey1.PublicKey,
			ExcludeFromOIDCDiscovery: true,
		},
		{
			KeyID:                    "key-2",
			PublicKey:                &rsaKey2.PublicKey,
			ExcludeFromOIDCDiscovery: false,
		},
	}

	backend.keyLock.Lock()
	backend.supportedKeys = supportedKeysT2
	backend.keyLock.Unlock()

	if _, err := plugin.keyCache.syncKeys(ctx); err != nil {
		t.Fatalf("Error while calling syncKeys: %v", err)
	}

	gotPubKeysT2 := plugin.keyCache.GetPublicKeys(ctx, "")
	if diff := cmp.Diff(gotPubKeysT2, wantPubKeysT2, cmpopts.SortSlices(sortPublicKeySlice)); diff != "" {
		t.Fatalf("Bad public keys; diff (-got +want)\n%s", diff)
	}
}

/*
Copyright 2020 The Kubernetes Authors.

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

package x509metrics

import (
	"crypto/x509"
	"errors"
	"net/http"
	"strings"

	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/component-base/metrics"
)

var _ utilnet.RoundTripperWrapper = &x509DeprecatedCertificateMetricsRTWrapper{}

type x509DeprecatedCertificateMetricsRTWrapper struct {
	rt http.RoundTripper

	missingSAN *metrics.Counter

	sha1 *metrics.Counter
}

// NewDeprecatedCertificateRoundTripperWrapperConstructor returns a RoundTripper wrapper that's usable within ClientConfig.Wrap.
//
// It increases the `missingSAN` counter whenever:
// 1. we get a x509.HostnameError with string `x509: certificate relies on legacy Common Name field`
//    which indicates an error caused by the deprecation of Common Name field when veryfing remote
//    hostname
// 2. the server certificate in response contains no SAN. This indicates that this binary run
//    with the GODEBUG=x509ignoreCN=0 in env
//
// It increases the `sha1` counter whenever:
// 1. we get a x509.InsecureAlgorithmError with string `SHA1`
//    which indicates an error caused by an insecure SHA1 signature
// 2. the server certificate in response contains a SHA1WithRSA or ECDSAWithSHA1 signature.
//    This indicates that this binary run with the GODEBUG=x509sha1=1 in env
func NewDeprecatedCertificateRoundTripperWrapperConstructor(missingSAN, sha1 *metrics.Counter) func(rt http.RoundTripper) http.RoundTripper {
	return func(rt http.RoundTripper) http.RoundTripper {
		return &x509DeprecatedCertificateMetricsRTWrapper{
			rt:         rt,
			missingSAN: missingSAN,
			sha1:       sha1,
		}
	}
}

func (w *x509DeprecatedCertificateMetricsRTWrapper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := w.rt.RoundTrip(req)

	if err != nil {
		checkForHostnameError(err, w.missingSAN)
		checkForSHA1InsecureAlgorithmError(err, w.sha1)
	} else if resp != nil {
		checkRespForNoSAN(resp, w.missingSAN)
		checkRespForSHA1(resp, w.sha1)
	}

	return resp, err
}

func (w *x509DeprecatedCertificateMetricsRTWrapper) WrappedRoundTripper() http.RoundTripper {
	return w.rt
}

// checkForHostnameError increases the metricCounter when we're running w/o GODEBUG=x509ignoreCN=0
// and the client reports a HostnameError about the legacy CN fields
func checkForHostnameError(err error, metricCounter *metrics.Counter) {
	if err != nil && errors.As(err, &x509.HostnameError{}) && strings.Contains(err.Error(), "x509: certificate relies on legacy Common Name field") {
		// increase the count of registered failures due to Go 1.15 x509 cert Common Name deprecation
		metricCounter.Inc()
	}
}

// checkRespForNoSAN increases the metricCounter when the server response contains
// a leaf certificate w/o the SAN extension
func checkRespForNoSAN(resp *http.Response, metricCounter *metrics.Counter) {
	if resp != nil && resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		if serverCert := resp.TLS.PeerCertificates[0]; !hasSAN(serverCert) {
			metricCounter.Inc()
		}
	}
}

func hasSAN(c *x509.Certificate) bool {
	sanOID := []int{2, 5, 29, 17}

	for _, e := range c.Extensions {
		if e.Id.Equal(sanOID) {
			return true
		}
	}
	return false
}

// checkForSHA1InsecureAlgorithmError increases the metricCounter when we're running w/o GODEBUG=x509sha1=1
// and the client reports an InsecureAlgorithmError about a SHA1 signature
func checkForSHA1InsecureAlgorithmError(err error, metricCounter *metrics.Counter) {
	var insecureAlgorithmError x509.InsecureAlgorithmError
	if err == nil {
		return
	}
	if !errors.As(err, &insecureAlgorithmError) {
		return
	}
	if strings.Contains(err.Error(), "SHA1") {
		// increase the count of registered failures due to Go 1.18 x509 sha1 signature deprecation
		metricCounter.Inc()
	}
}

// checkRespForSHA1 increases the metricCounter when the server response contains
// a leaf certificate with a deprecated SHA1 signature
func checkRespForSHA1(resp *http.Response, metricCounter *metrics.Counter) {
	if resp != nil && resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		if serverCert := resp.TLS.PeerCertificates[0]; serverCert.SignatureAlgorithm == x509.SHA1WithRSA || serverCert.SignatureAlgorithm == x509.ECDSAWithSHA1 {
			metricCounter.Inc()
		}
	}
}

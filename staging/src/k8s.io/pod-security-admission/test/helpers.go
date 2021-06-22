package test

import (
	corev1 "k8s.io/api/core/v1"
)

// tweak makes a copy of in, passes it to f(), and returns the result.
// the input is not modified.
func tweak(in *corev1.Pod, f func(copy *corev1.Pod)) *corev1.Pod {
	out := in.DeepCopy()
	f(out)
	return out
}

// ensureSecurityContext ensures the pod and all initContainers and containers have a non-nil security context.
func ensureSecurityContext(p *corev1.Pod) *corev1.Pod {
	p = p.DeepCopy()
	if p.Spec.SecurityContext == nil {
		p.Spec.SecurityContext = &corev1.PodSecurityContext{}
	}
	for i := range p.Spec.Containers {
		if p.Spec.Containers[i].SecurityContext == nil {
			p.Spec.Containers[i].SecurityContext = &corev1.SecurityContext{}
		}
	}
	for i := range p.Spec.InitContainers {
		if p.Spec.InitContainers[i].SecurityContext == nil {
			p.Spec.InitContainers[i].SecurityContext = &corev1.SecurityContext{}
		}
	}
	return p
}

// ensureSELinuxOptions ensures the pod and all initContainers and containers have a non-nil seLinuxOptions.
func ensureSELinuxOptions(p *corev1.Pod) *corev1.Pod {
	p = ensureSecurityContext(p)
	if p.Spec.SecurityContext.SELinuxOptions == nil {
		p.Spec.SecurityContext.SELinuxOptions = &corev1.SELinuxOptions{}
	}
	for i := range p.Spec.Containers {
		if p.Spec.Containers[i].SecurityContext.SELinuxOptions == nil {
			p.Spec.Containers[i].SecurityContext.SELinuxOptions = &corev1.SELinuxOptions{}
		}
	}
	for i := range p.Spec.InitContainers {
		if p.Spec.InitContainers[i].SecurityContext.SELinuxOptions == nil {
			p.Spec.InitContainers[i].SecurityContext.SELinuxOptions = &corev1.SELinuxOptions{}
		}
	}
	return p
}

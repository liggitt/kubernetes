/*
Copyright 2025 The Kubernetes Authors.

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

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/pod-security-admission/admission"
	"k8s.io/pod-security-admission/api"
	"k8s.io/pod-security-admission/policy"
)

var usage = fmt.Sprintf("Usage: %s --level=[baseline | restricted] --version=[latest | v1.x] <file> [file...]\n", filepath.Base(os.Args[0]))

func main() {
	level := "restricted"
	version := "latest"
	flag.StringVar(&level, "level", level, "Policy level to evaluate. 'baseline' or 'restricted'")
	flag.StringVar(&version, "version", version, "Policy version to evaluate. 'latest' or 'v1.x'")
	flag.Parse()

	apiLevel, err := api.ParseLevel(level)
	exitOnErr(err, usage, 2)
	apiVersion, err := api.ParseVersion(version)
	exitOnErr(err, usage, 2)
	args := flag.Args()
	if len(args) == 0 {
		exitOnErr(fmt.Errorf("no files specified"), usage, 2)
	}

	extractor := admission.DefaultPodSpecExtractor{}
	evaluator, err := policy.NewEvaluator(policy.DefaultChecks())
	exitOnErr(err, "", 1)

	builder := resource.NewLocalBuilder().
		WithScheme(scheme.Scheme, scheme.Scheme.PrioritizedVersionsAllGroups()...).
		FilenameParam(false, &resource.FilenameOptions{Recursive: true, Filenames: args}).
		Flatten().
		ContinueOnError()

	fmt.Printf("evaluating pod security standards level=%s, version=%s\n\n", apiLevel, apiVersion)

	failedEvaluation := false
	err = builder.Do().IgnoreErrors(func(err error) bool {
		return strings.Contains(err.Error(), "no kind") // ignore errors about unknown kinds
	}).Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}
		meta, spec, err := extractor.ExtractPodSpec(info.Object)
		if err != nil {
			return nil // type doesn't have a pod spec
		}
		if spec == nil {
			return nil // specific object doesn't have a pod spec
		}
		result := policy.AggregateCheckResults(evaluator.EvaluatePod(api.LevelVersion{Level: apiLevel, Version: apiVersion}, meta, spec))
		source := ""
		if len(info.Source) > 0 {
			source = info.Source + ": "
		}
		if result.Allowed {
			fmt.Printf("%s%s %q passed\n", source, info.Object.GetObjectKind().GroupVersionKind().Kind, info.Name)
			return nil
		}
		failedEvaluation = true
		fmt.Printf("%s%s %q failed:\n", source, info.Object.GetObjectKind().GroupVersionKind().Kind, info.Name)
		for i := range result.ForbiddenReasons {
			fmt.Printf("   %s: %s\n", result.ForbiddenReasons[i], result.ForbiddenDetails[i])
		}
		return nil
	})
	exitOnErr(err, "", 1)
	if failedEvaluation {
		os.Exit(1)
	}
}

func exitOnErr(err error, msg string, code int) {
	if err == nil {
		return
	}
	fmt.Println(msg)
	fmt.Println(err)
	os.Exit(code)
}

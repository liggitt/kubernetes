load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["terminal.go"],
    importmap = "k8s.io/kubernetes/vendor/golang.org/x/crypto/ssh/terminal",
    importpath = "golang.org/x/crypto/ssh/terminal",
    visibility = ["//visibility:public"],
    deps = ["//vendor/golang.org/x/term:go_default_library"],
)

filegroup(
    name = "package-srcs",
    srcs = glob(["**"]),
    tags = ["automanaged"],
    visibility = ["//visibility:private"],
)

filegroup(
    name = "all-srcs",
    srcs = [":package-srcs"],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)

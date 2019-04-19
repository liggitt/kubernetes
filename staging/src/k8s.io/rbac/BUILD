filegroup(
    name = "package-srcs",
    srcs = glob(["**"]),
    tags = ["automanaged"],
    visibility = ["//visibility:private"],
)

filegroup(
    name = "all-srcs",
    srcs = [
        ":package-srcs",
        "//staging/src/k8s.io/rbac/authorizer:all-srcs",
        "//staging/src/k8s.io/rbac/helpers:all-srcs",
        "//staging/src/k8s.io/rbac/reconciliation:all-srcs",
        "//staging/src/k8s.io/rbac/validation:all-srcs",
    ],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)

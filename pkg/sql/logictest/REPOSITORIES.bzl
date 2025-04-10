# DO NOT EDIT THIS FILE MANUALLY! Use `release update-releases-file`.
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

CONFIG_LINUX_AMD64 = "linux-amd64"
CONFIG_LINUX_ARM64 = "linux-arm64"
CONFIG_DARWIN_AMD64 = "darwin-10.9-amd64"
CONFIG_DARWIN_ARM64 = "darwin-11.0-arm64"

_CONFIGS = [
    ("24.3.9", [
        (CONFIG_DARWIN_AMD64, "a771b186ef1618345b59f10ad9ea8932bd6f983b55f8c977ed46692a5bae2b1d"),
        (CONFIG_DARWIN_ARM64, "2c40295a9474d0639b55fc1478dcd993e789939fe17ab7bf1c618757c007a4c5"),
        (CONFIG_LINUX_AMD64, "18cdb9717bdbfd730a77db3a8afea3394cebad44de26bfc3bf7cc238a31dd73b"),
        (CONFIG_LINUX_ARM64, "26d23adf2f0f8481f445a6290126222b62061cb065db9c1c918c85ac53ad7e31"),
    ]),
    ("25.1.3", [
        (CONFIG_DARWIN_AMD64, "dbddf0cdf95147f9d2a57da74b62d0572febed001a252a687fe060f1affdb434"),
        (CONFIG_DARWIN_ARM64, "c5e54bf53f716e993365498e990ea5894a902cea94faa71538eade8d048ffdd4"),
        (CONFIG_LINUX_AMD64, "ba97aafe1a0b2c892d49d8c1dacf1c2e7eb2ae312cc30e50d0891fb92a5975c1"),
        (CONFIG_LINUX_ARM64, "1db6a1f964a8e0cf44d2aa4c810fb07c21313604532cf3780bf16c2822d8b258"),
    ]),
]

def _munge_name(s):
    return s.replace("-", "_").replace(".", "_")

def _repo_name(version, config_name):
    return "cockroach_binary_v{}_{}".format(
        _munge_name(version),
        _munge_name(config_name))

def _file_name(version, config_name):
    return "cockroach-v{}.{}/cockroach".format(
        version, config_name)

def target(config_name):
    targets = []
    for versionAndConfigs in _CONFIGS:
        version, _ = versionAndConfigs
        targets.append("@{}//:{}".format(_repo_name(version, config_name),
                                         _file_name(version, config_name)))
    return targets

def cockroach_binaries_for_testing():
    for versionAndConfigs in _CONFIGS:
        version, configs = versionAndConfigs
        for config in configs:
            config_name, shasum = config
            file_name = _file_name(version, config_name)
            http_archive(
                name = _repo_name(version, config_name),
                build_file_content = """exports_files(["{}"])""".format(file_name),
                sha256 = shasum,
                urls = [
                    "https://binaries.cockroachdb.com/{}".format(
                        file_name.removesuffix("/cockroach")) + ".tgz",
                ],
            )

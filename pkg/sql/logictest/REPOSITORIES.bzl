# DO NOT EDIT THIS FILE MANUALLY! Use `release update-releases-file`.
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

CONFIG_LINUX_AMD64 = "linux-amd64"
CONFIG_LINUX_ARM64 = "linux-arm64"
CONFIG_DARWIN_AMD64 = "darwin-10.9-amd64"
CONFIG_DARWIN_ARM64 = "darwin-11.0-arm64"

_CONFIGS = [
    ("24.2.6", [
        (CONFIG_DARWIN_AMD64, "402473e32e26933ac00c25afe2cbced6496e71c16161cd58960e423dbb765424"),
        (CONFIG_DARWIN_ARM64, "1aaa13bb1537f6b848c2cffafa716fa24cca64318458bb20a034d1cd99dd73df"),
        (CONFIG_LINUX_AMD64, "57bfd75743e36ffa0f38ebdd6ec6e1ee4a7934612387550cb07f7a9d241e6742"),
        (CONFIG_LINUX_ARM64, "ef8793204f20e83d7bede63bb5dbf0f45659c9a8940912adfb5dbef7e5d29386"),
    ]),
    ("24.3.1", [
        (CONFIG_DARWIN_AMD64, "fb54989d77fdaeaa6a24f9b5a791fc56e662fdc67447c692bf0067db7d46b94f"),
        (CONFIG_DARWIN_ARM64, "bda025edcd9f6671879872e8a6fdeab6b9899f10d56a883b93dc58df2d2e4777"),
        (CONFIG_LINUX_AMD64, "3665ad0dad28d2dc6b16017aa57b8384c2fc39e57b79878df3287a36da3bff6f"),
        (CONFIG_LINUX_ARM64, "1691215bd43809334cccf0cd93f714ca782634247ad99695660fe33c9430da54"),
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

# Options

## Configuration

Options for Conftest can be configured on the command line as flag arguments, environment variables, or a configuration file.

When multiple configuration sources are present, Conftest will process the configuration values in the following order:

1. Flag Arguments
1. Environment Variables
1. Configuration File

When using environment variables, the environment variable should be the same name as the flag, prefixed with `CONFTEST_`. For example, to set the policy directory, the environment variable would be `CONFTEST_POLICY`.

When using a configuration file, the configuration file should be in the working directory for Conftest and named `conftest.toml`. An example can be found below:

```toml
# You can override the directory in which to store and look for policies
policy = "tests"

# You can override the namespace which to search for rules
namespace = "conftest"
```

## `--combine`

This flag introduces *BREAKING CHANGES* in how Conftest provides input to rego policies. However, you may find it useful to use as it allows you to compare multiple values from different configurations simultaneously.

The `--combine` flag combines files into one `input` data structure. The structure is an `array` where each element is a `map` with two keys: a `path` key with the relative file path of the file being evaluated and a `contents` key containing the actual document.

Let's try it!

Save the following as `policy/combine.rego`:

```rego
package main

deny[msg] {
  input[i].contents.kind == "Deployment"

  deployment := input[i].contents

  not service_selects_app(deployment.spec.selector.matchLabels.app)

  msg := sprintf("Deployment %v with selector %v does not match any Services", [deployment.metadata.name, deployment.spec.selector.matchLabels])
}

service_selects_app(app) {
  input[service].contents.kind == "Service"
  input[service].contents.spec.selector.app == app
}
```

Save the following as `manifests.yaml`:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: hello-kubernetes
spec:
  type: LoadBalancer
  ports:
  - port: 80
    targetPort: 8080
  selector:
    app: goodbye-kubernetes
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hello-kubernetes
spec:
  replicas: 3
  selector:
    matchLabels:
      app: hello-kubernetes
```

With the above file created, run the following command:

```console
$ conftest test manifests.yaml --combine

FAIL - Combined - Deployment hello-kubernetes has selector hello-kubernetes that does not match any Services

1 test, 0 passed, 0 warnings, 1 failure, 0 exceptions
```

It is also possible to pass in multiple files. Creating `.yaml` file for both the `Service` and the `Deployment`.

```console
$ conftest test service.yaml deployment.yaml --combine

FAIL - Combined - Deployment hello-kubernetes has selector hello-kubernetes that does not match any Services

1 test, 0 passed, 0 warnings, 1 failure, 0 exceptions
```

This is just the tip of the iceberg. Now you can ensure that duplicate values match across the entirety of your configuration files.

## `--data`

Sometimes policies require additional data in order to determine an answer.

For example, an allowed list of resources that can be created. Instead of hardcoding this information inside of your policy, conftest allows passing paths to data files with the `--data` flag.

```console
conftest test -p examples/data/policy -d examples/data/exclusions examples/data/service.yaml
```

The paths at the flag are recursively searched for JSON and YAML files. Data can be imported as follows:

Given the following yaml file:

```yaml
services:
  ports:
  - 22
```

This can be imported into your policy:

```rego
import data.services

ports := services.ports
```

## `--fail-on-warn`

Policies can either be catagorized as a warning (using the `warn` rule) or a failure (using the `deny` or `violation` rules). By default, Conftest only returns an exit code of `1` when a policy has failed.

The `--fail-on-warn` flag changes this behavior to the following:

- Exit code of 0: No failures or warnings.
- Exit code of 1: No failures, but there exists at least one warning.
- Exit code of 2: At least one failure.

## `--ignore`

When a directory is given as an input, Conftest will recursively find, and test all files that it supports. To ignore certain directories or files, the `--ignore` flag takes a regexp pattern that will ignore directories and files that match the pattern.

Here are some examples:

```console
conftest test -p examples/test/ test/ --ignore="parent/"
conftest test -p examples/test/ test/ --ignore="child/"
conftest test -p examples/test/ test/ --ignore=".*.yaml"
conftest test -p examples/test/ test/ --ignore=".*.cue|.*.yaml"
```

## `--output`

The output of Conftest can be configured using the `--output` flag (`-o`).

As of today Conftest supports the following output types:

- Plaintext `--output=stdout`
- JSON: `--output=json`
- [TAP](https://testanything.org/): `--output=tap`
- Table `--output=table`
- JUnit `--output=junit`
- GitHub `--output=github`

### Plaintext

```console
$ conftest test -p examples/kubernetes/policy examples/kubernetes/deployment.yaml
FAIL - examples/kubernetes/deployment.yaml - hello-kubernetes must include Kubernetes recommended labels: https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/#labels     
FAIL - examples/kubernetes/deployment.yaml - Containers must not run as root in Deployment hello-kubernetes
FAIL - examples/kubernetes/deployment.yaml - Deployment hello-kubernetes must provide app/release labels for pod selectors
FAIL - examples/kubernetes/deployment.yaml - Found deployment hello-kubernetes but deployments are not allowed

5 tests, 1 passed, 0 warnings, 4 failures, 0 exceptions
```

### JSON

```console
$ conftest test -o json -p examples/kubernetes/policy examples/kubernetes/deployment.yaml
[
        {
                "filename": "examples/kubernetes/deployment.yaml",
                "successes": 1,
                "failures": [
                        {
                                "msg": "Containers must not run as root in Deployment hello-kubernetes"
                        },
                        {
                                "msg": "Deployment hello-kubernetes must provide app/release labels for pod selectors"
                        },
                        {
                                "msg": "hello-kubernetes must include Kubernetes recommended labels: https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/#labels"
                        },
                        {
                                "msg": "Found deployment hello-kubernetes but deployments are not allowed",
                                "metadata": {
                                        "details": {}
                                }
                        }
                ]
        }
]
```

### TAP

```console
$ conftest test -o tap -p examples/kubernetes/policy examples/kubernetes/deployment.yaml
1..5
not ok 1 - examples/kubernetes/deployment.yaml - Found deployment hello-kubernetes but deployments are not allowed
not ok 2 - examples/kubernetes/deployment.yaml - Containers must not run as root in Deployment hello-kubernetes
not ok 3 - examples/kubernetes/deployment.yaml - Deployment hello-kubernetes must provide app/release labels for pod selectors
not ok 4 - examples/kubernetes/deployment.yaml - hello-kubernetes must include Kubernetes recommended labels: https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/#labels
# successes
ok 5 - examples/kubernetes/deployment.yaml -
```

### Table

```console
$ conftest test -p examples/kubernetes/policy examples/kubernetes/service.yaml -o table
+---------+----------------------------------+--------------------------------+
| RESULT  |               FILE               |            MESSAGE             |
+---------+----------------------------------+--------------------------------+
| success | examples/kubernetes/service.yaml |                                |
| success | examples/kubernetes/service.yaml |                                |
| success | examples/kubernetes/service.yaml |                                |
| success | examples/kubernetes/service.yaml |                                |
| warning | examples/kubernetes/service.yaml | Found service hello-kubernetes |
|         |                                  | but services are not allowed   |
+---------+----------------------------------+--------------------------------+
```

### JUnit

```console
$ conftest test -p examples/kubernetes/policy examples/kubernetes/service.yaml -o junit
<?xml version="1.0" encoding="UTF-8"?>
<testsuites>
        <testsuite tests="5" failures="4" time="0.000" name="conftest">
                <properties>
                        <property name="go.version" value="go1.15.1"></property>
                </properties>
                <testcase classname="conftest" name="examples/kubernetes/deployment.yaml - Containers must not run as root in Deployment hello-kubernetes" time="0.000">
                        <failure message="Failed" type="">Containers must not run as root in Deployment hello-kubernetes</failure>
                </testcase>
                <testcase classname="conftest" name="examples/kubernetes/deployment.yaml - Deployment hello-kubernetes must provide app/release labels for pod selectors" time="0.000">
                        <failure message="Failed" type="">Deployment hello-kubernetes must provide app/release labels for pod selectors</failure>
                </testcase>
                <testcase classname="conftest" name="examples/kubernetes/deployment.yaml - hello-kubernetes must include Kubernetes recommended labels: https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/#labels" time="0.000">
                        <failure message="Failed" type="">hello-kubernetes must include Kubernetes recommended labels: https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/#labels</failure>
                </testcase>
                <testcase classname="conftest" name="examples/kubernetes/deployment.yaml - Found deployment hello-kubernetes but deployments are not allowed" time="0.000">
                        <failure message="Failed" type="">Found deployment hello-kubernetes but deployments are not allowed</failure>
                </testcase>
                <testcase classname="conftest" name="examples/kubernetes/deployment.yaml - " time="0.000"></testcase>
        </testsu
```

### GitHub

```console
$ conftest test -o github -p examples/kubernetes/policy examples/kubernetes/deployment.yaml
::group::Testing 'examples/kubernetes/deployment.yaml' against 5 policies in namespace 'main'
::error file=examples/kubernetes/deployment.yaml::Containers must not run as root in Deployment hello-kubernetes
::error file=examples/kubernetes/deployment.yaml::Deployment hello-kubernetes must provide app/release labels for pod selectors
::error file=examples/kubernetes/deployment.yaml::hello-kubernetes must include Kubernetes recommended labels: https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/#labels
::error file=examples/kubernetes/deployment.yaml::Found deployment hello-kubernetes but deployments are not allowed
success file=examples/kubernetes/deployment.yaml 1
::endgroup::
5 tests, 1 passed, 0 warnings, 4 failures, 0 exceptions
```

Use Conftest directly to check incoming Pull Requests in GitHub:

```yaml
---
name: Conftest

on:
  pull_request:
    branches: 
      - main

jobs:
  conftest:
    runs-on: ubuntu-latest
    container: openpolicyagent/conftest:latest
    steps:
      - name: Code checkout
        uses: actions/checkout@v2
      - name: Validate Kubernetes policy
        run: |
          conftest test -o github -p examples/kubernetes/policy examples/kubernetes/deployment.yaml
```

## `--parser`

Conftest normally detects which parser to used based on the file extension of the file, even when multiple input files are passed in. However, it is possible force a specific parser to be used with the `--parser` flag.

For the available parsers, take a look at [parsers](https://github.com/open-policy-agent/conftest/tree/master/parser).

For instance:

```console
$ conftest test -p examples/hcl1/policy examples/hcl1/gke.tf --parser hcl2

2 tests, 2 passed, 0 warnings, 0 failures, 0 exceptions
```

## `--policy`

Conftest will, by default, look for policies in the `policy` folder. This can be changed with the `--policy` (or `-p`) flag. 

This flag can also be repeated, if you have multiple directories with policy files. For example one with organization-wide policies, and another one with team-specific policies. Keep in mind when using multiple directories that the rules are treated as if they had all been read from the same directory, so make sure any rules sharing the same name are compatible.

Read rules from the policy directory (default):
```console
$ conftest test files/
```

Read rules from the `my-policies` directory:

```console
$ conftest test -p my-policies files/
```

Reads rules from the `my-policies` and `org-policies` directories:

```console
$ conftest test -p my-policies -p org-policies files/
```

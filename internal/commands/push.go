package commands

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	auth "github.com/deislabs/oras/pkg/auth/docker"
	"github.com/deislabs/oras/pkg/content"
	"github.com/deislabs/oras/pkg/oras"
	"github.com/open-policy-agent/conftest/policy"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/cobra"
)

const pushDesc = `
This command uploads Open Policy Agent bundles to an OCI registry

Storing policies in OCI registries is similar to how Docker containers are stored.
With conftest, Rego policies are bundled and pushed to the OCI registry e.g.:

	$ conftest push instrumenta.azurecr.io/my-registry

Optionally, a tag can be specified, e.g.:

	$ conftest push instrumenta.azurecr.io/my-registry:v1

Optionally, specific directory can be passed as second argument, e.g.:

	$ conftest push instrumenta.azurecr.io/my-registry:v1 /path/to/dir

If no tag is passed, by default, conftest uses the 'latest' tag. The policies can be retrieved
using the pull command, e.g.:

	$ conftest pull instrumenta.azurecr.io/my-registry:v1

Alternatively, the policies can be pulled as part of running the test command:

	$ conftest test --update instrumenta.azurecr.io/my-registry:v1 <my-input-file>

Conftest leverages the ORAS library under the hood. This allows arbitrary artifacts to 
be stored in compatible OCI registries. Currently open policy agent bundles are supported by 
the docker/distribution (https://github.com/docker/distribution) registry and by Azure.

The policy location defaults to the policy directory in the local folder.
The location can be overridden with the '--policy' flag, e.g.:

	$ conftest push --policy <my-directory> <oci-url>
`

const (
	openPolicyAgentConfigMediaType      = "application/vnd.cncf.openpolicyagent.config.v1+json"
	openPolicyAgentPolicyLayerMediaType = "application/vnd.cncf.openpolicyagent.policy.layer.v1+rego"
	openPolicyAgentDataLayerMediaType   = "application/vnd.cncf.openpolicyagent.data.layer.v1+json"
)

// NewPushCommand creates a new push command which allows users to push
// bundles to an OCI registry
func NewPushCommand(ctx context.Context, logger *log.Logger) *cobra.Command {
	cmd := cobra.Command{
		Use:   "push <repository> [filepath]",
		Short: "Upload OPA bundles to an OCI registry",
		Long:  pushDesc,
		Args:  cobra.RangeArgs(1, 2),

		RunE: func(cmd *cobra.Command, args []string) error {
			var path string
			if len(args) == 2 {
				path = args[1]
			} else {
				var err error
				path, err = os.Getwd()
				if err != nil {
					return fmt.Errorf("get working directory: %w", err)
				}
			}

			repository := args[0]

			logger.Printf("pushing bundle to: %s\n", repository)
			manifest, err := pushBundle(ctx, repository, path)
			if err != nil {
				return fmt.Errorf("push bundle: %w", err)
			}
			logger.Printf("pushed bundle with digest: %s\n", manifest.Digest)

			return nil
		},
	}

	return &cmd
}

func pushBundle(ctx context.Context, repository string, path string) (*ocispec.Descriptor, error) {
	cli, err := auth.NewClient()
	if err != nil {
		return nil, fmt.Errorf("get auth client: %w", err)
	}

	resolver, err := cli.Resolver(ctx, http.DefaultClient, false)
	if err != nil {
		return nil, fmt.Errorf("docker resolver: %w", err)
	}

	memoryStore := content.NewMemoryStore()
	layers, err := buildLayers(ctx, memoryStore, path)
	if err != nil {
		return nil, fmt.Errorf("building layers: %w", err)
	}

	var repositoryWithTag string
	if strings.Contains(repository, ":") {
		repositoryWithTag = repository
	} else {
		repositoryWithTag = repository + ":latest"
	}

	extraOpts := []oras.PushOpt{oras.WithConfigMediaType(openPolicyAgentConfigMediaType)}
	manifest, err := oras.Push(ctx, resolver, repositoryWithTag, memoryStore, layers, extraOpts...)
	if err != nil {
		return nil, fmt.Errorf("pushing manifest: %w", err)
	}

	return &manifest, nil
}

func buildLayers(ctx context.Context, memoryStore *content.Memorystore, path string) ([]ocispec.Descriptor, error) {
	root, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("get abs path: %w", err)
	}

	loader := policy.Loader{
		PolicyPaths: []string{root},
		DataPaths:   []string{root},
	}

	engine, err := loader.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("load: %w", err)
	}

	policyLayers, err := buildLayer(engine.Policies(), root, memoryStore, openPolicyAgentPolicyLayerMediaType)
	if err != nil {
		return nil, fmt.Errorf("build policy layer: %w", err)
	}

	dataLayers, err := buildLayer(engine.Documents(), root, memoryStore, openPolicyAgentDataLayerMediaType)
	if err != nil {
		return nil, fmt.Errorf("build data layer: %w", err)
	}

	layers := append(policyLayers, dataLayers...)
	return layers, nil
}

func buildLayer(files map[string]string, root string, memoryStore *content.Memorystore, mediaType string) ([]ocispec.Descriptor, error) {
	var layer ocispec.Descriptor
	var layers []ocispec.Descriptor

	for path, contents := range files {
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return nil, fmt.Errorf("get relative filepath: %w", err)
		}

		path := filepath.ToSlash(relative)

		layer = memoryStore.Add(path, mediaType, []byte(contents))
		layers = append(layers, layer)
	}

	return layers, nil
}

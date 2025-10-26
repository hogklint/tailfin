package tailfincmd

import (
	"github.com/containerd/log"
	cliconfig "github.com/docker/cli/cli/config"
	ctxdocker "github.com/docker/cli/cli/context/docker"
	ctxstore "github.com/docker/cli/cli/context/store"
	dockerclient "github.com/docker/docker/client"
	sdkcontext "github.com/docker/go-sdk/context"
)

func getContextStore() *ctxstore.ContextStore {
	storeConfigLazy := ctxstore.NewConfig(
		func() interface{} { return &ctxdocker.EndpointMeta{} },
		ctxstore.EndpointTypeGetter(ctxdocker.DockerEndpoint, func() interface{} { return &ctxdocker.EndpointMeta{} }),
	)

	return ctxstore.New(cliconfig.ContextStoreDir(), storeConfigLazy)
}

func getContextEndpoint(context string, contextStore *ctxstore.ContextStore) (ctxdocker.Endpoint, error) {
	metadata, err := contextStore.GetMetadata(context)
	if err != nil {
		return ctxdocker.Endpoint{}, err
	}

	endpoint, err := ctxdocker.EndpointFromContext(metadata)
	if err != nil {
		return ctxdocker.Endpoint{}, err
	}

	tlsEndpoint, err := ctxdocker.WithTLSData(contextStore, context, endpoint)
	if err != nil {
		return ctxdocker.Endpoint{}, err
	}
	return tlsEndpoint, nil
}

func getClientOpts(context string) ([]dockerclient.Opt, error) {
	if context == sdkcontext.DefaultContextName {
		return []dockerclient.Opt{dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation()}, nil
	}

	contextStore := getContextStore()
	endpoint, err := getContextEndpoint(context, contextStore)
	if err != nil {
		return []dockerclient.Opt{}, err
	}

	clientOpts, err := endpoint.ClientOpts()
	if err != nil {
		return []dockerclient.Opt{}, err
	}

	log.L.Infof("Docker context %s - connecting to %s", context, endpoint.Host)
	return clientOpts, nil
}

func getDockerClient(flagContext string) (*dockerclient.Client, error) {
	context := flagContext
	if context == "" {
		var err error
		context, err = sdkcontext.Current()
		if err != nil {
			return nil, err
		}
	}
	log.L.Infof("Using docker context %s", context)

	clientOpts, err := getClientOpts(context)
	if err != nil {
		return nil, err
	}

	client, err := dockerclient.NewClientWithOpts(clientOpts...)
	if err != nil {
		return nil, err
	}

	return client, nil
}

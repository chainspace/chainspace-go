package contracts // import "chainspace.io/prototype/contracts"

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"

	"chainspace.io/prototype/config"
	"chainspace.io/prototype/log"
	"chainspace.io/prototype/log/fld"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

type Contracts struct {
	client  *client.Client
	configs *config.Contracts
}

func (c *Contracts) ensureImageExists(cfg config.DockerContract, images []types.ImageSummary) error {
	ok := false
	for _, image := range images {
		for _, tag := range image.RepoTags {
			if cfg.Image == tag {
				ok = true
				break
			}
		}
	}
	if !ok {
		return fmt.Errorf("unable to find the docker image '%v', please pull the image then restart the chainspace node", cfg.Image)
	}
	return nil
}

func (c *Contracts) createContainer(cfg config.DockerContract) (string, error) {
	port := nat.PortSet(map[nat.Port]struct{}{nat.Port(cfg.Port + "/tcp"): struct{}{}})
	name := fmt.Sprintf("chainspace-%v", cfg.Name)

	portmap := nat.PortMap(map[nat.Port][]nat.PortBinding{
		nat.Port(cfg.Port + "/tcp"): []nat.PortBinding{
			{
				HostIP:   "0.0.0.0",
				HostPort: cfg.HostPort,
			},
		},
	})
	body, err := c.client.ContainerCreate(
		context.Background(),
		&container.Config{
			Image:        cfg.Image,
			ExposedPorts: port,
		},
		&container.HostConfig{
			PortBindings: portmap,
		},
		nil,
		name,
	)

	if err != nil {
		return "", fmt.Errorf("unable to create container, %v", err)
	}

	log.Info("new contract container created", log.String("container.name", name), log.String("container.id", body.ID))
	for _, v := range body.Warnings {
		log.Info("creation warning", log.String("warning", v))
	}

	return body.ID, nil
}

func (c *Contracts) containerExists(cfg config.DockerContract, containers []types.Container) (string, string, bool) {
	name := fmt.Sprintf("/chainspace-%v", cfg.Name)

	for _, container := range containers {
		for _, v := range container.Names {
			if name == v {
				log.Info("Container already exists", log.String("container.name", name), log.String("container.state", container.State))
				return container.ID, container.State, true
			}
		}
	}

	return "", "", false
}

func (c *Contracts) startContainer(cfg config.DockerContract, images []types.ImageSummary, containers []types.Container) error {
	// check if the image exists
	var err error
	if err = c.ensureImageExists(cfg, images); err != nil {
		return err
	}

	// create container if not exists
	containerID, state, ok := c.containerExists(cfg, containers)
	if !ok {
		containerID, err = c.createContainer(cfg)
		if err != nil {
			return err
		}
	}

	if state != "running" {
		err = c.client.ContainerStart(context.Background(), containerID, types.ContainerStartOptions{})
	}

	return nil
}

func (c *Contracts) Start() error {
	// get images list
	images, err := c.client.ImageList(context.Background(), types.ImageListOptions{})
	if err != nil {
		return fmt.Errorf("unable to load docker image list, %v", err)
	}

	// get containers list
	containers, err := c.client.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		return fmt.Errorf("unable to load docker containers lists, %v", err)
	}

	for _, v := range c.configs.DockerContracts {
		if err := c.startContainer(v, images, containers); err != nil {
			return err
		}
	}
	return nil
}

func (c *Contracts) stopContainer(cfg config.DockerContract, containers []types.Container) {
	name := fmt.Sprintf("/chainspace-%v", cfg.Name)
	for _, container := range containers {
		for _, v := range container.Names {
			if name == v {
				if err := c.client.ContainerStop(context.Background(), container.ID, nil); err != nil {
					log.Error("unable to stop container", log.String("container.io", container.ID), fld.Err(err))
				} else {
					log.Info("contract container stopped", log.String("container.id", container.ID))
					if err := c.client.ContainerRemove(context.Background(), container.ID, types.ContainerRemoveOptions{}); err != nil {
						log.Error("unable to remove container", log.String("container.id", container.ID), fld.Err(err))
					} else {
						log.Info("contract container removed", log.String("container.id", container.ID))
					}
				}
				return
			}
		}
	}

}

func (c *Contracts) Stop() error {
	containers, err := c.client.ContainerList(
		context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		return fmt.Errorf("unable to load docker containers lists, %v", err)
	}

	for _, v := range c.configs.DockerContracts {
		c.stopContainer(v, containers)
	}

	return nil
}

func (c *Contracts) callHealthCheck(addr string) error {
	resp, err := http.Get(addr)
	if err != nil {
		return err
	}

	b, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return fmt.Errorf("error reading response from healthcheck: %v", err.Error())
	}
	res := struct {
		OK bool `json:"ok"`
	}{}
	err = json.Unmarshal(b, &res)
	if err != nil {
		return fmt.Errorf("unable to unmarshal response from healthcheck: %v", err.Error())
	}
	if res.OK != true {
		return fmt.Errorf("invalid healthcheck response")
	}

	return nil
}

func (c *Contracts) EnsureUp() error {
	for _, contract := range c.configs.DockerContracts {
		u, err := url.Parse(fmt.Sprintf("http://0.0.0.0:%v", contract.HostPort))
		if err != nil {
			log.Fatal("unable to parse docker contract url", log.String("contract.name", contract.Name))
		}
		u.Path = path.Join(u.Path, contract.HealthCheckURL)
		addr := u.String()
		log.Info("calling docker contract healthcheck", log.String("contract.name", contract.Name), log.String("contract.healtcheckurl", addr))
		if err := c.callHealthCheck(addr); err != nil {
			return fmt.Errorf("docker contract '%v' is unavailable, %v", contract.Name, err)
		}
	}

	for _, contract := range c.configs.Contracts {
		u, err := url.Parse(contract.Addr)
		if err != nil {
			log.Fatal("unable to parse contract url", log.String("contract.name", contract.Name))
		}
		u.Path = path.Join(u.Path, contract.HealthCheckURL)
		addr := u.String()
		log.Info("calling contract healthcheck", log.String("contract.name", contract.Name), log.String("contract.healtcheckurl", addr))
		if err := c.callHealthCheck(addr); err != nil {
			return fmt.Errorf("contract '%v' is unavailable, %v", contract.Name, err)
		}

	}
	return nil
}

func (c *Contracts) GetCheckers() []Checker {
	checkers := []Checker{}
	for _, v := range c.configs.DockerContracts {
		checkers = append(checkers, NewDockerCheckers(&v)...)
	}
	for _, v := range c.configs.Contracts {
		checkers = append(checkers, NewCheckers(&v)...)
	}
	return checkers
}

func New(cfg *config.Contracts) (*Contracts, error) {
	c, err := client.NewClientWithOpts(
		client.WithVersion(cfg.DockerMinimalVersion),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to instantiate the docker client, %v", err)
	}

	return &Contracts{
		configs: cfg,
		client:  c,
	}, nil
}

package main

import (
	"cubes/instance"
	"fmt"
	"github.com/urfave/cli"
	"log"
	"os"
	"strconv"
	"strings"
)

func initProject(c *cli.Context) error {
	return nil
}

func main() {
	app := cli.NewApp()
	app.Commands = []cli.Command{
		{

			Name:   "init",
			Usage:  "init project",
			Action: initProject,
		},
		{
			Name:  "instance",
			Usage: "cube instance",
			Subcommands: []cli.Command{
				{
					Name:  "add",
					Usage: "adds cube instance",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "channels",
							Usage: "channels mapping: --channels 'cubeChannel1:handlerChannel1;cubeChannel2:handlerChannel2'",
						},
						cli.StringFlag{
							Name:  "ports",
							Usage: "ports mapping: --ports 'hostPort:handlerPort:protocol;80:8080:tcp'",
						},
						cli.StringFlag{
							Name:  "params",
							Usage: "params: --params 'param1:Value1;param2:Value2'",
						},
					},
					ArgsUsage: "[-ports] [-channels] name source",
					Action:    instanceAdd,
				},
				{
					Name:      "remove",
					Usage:     "remove cube instance",
					ArgsUsage: "name",
					Action:    instanceRemove,
				},
				{
					Name:  "start",
					Usage: "start cube instance",
					Action: func(c *cli.Context) error {
						log.Println("start instance")
						return nil
					},
				},
				{
					Name:  "stop",
					Usage: "stops cube instance",
					Action: func(c *cli.Context) error {
						log.Println("stop instance")
						return nil
					},
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}


func parseChannelsMapping(channelsMappingRaw string) (*map[string]string, error) {
	channelsMapping := map[string]string{}

	if channelsMappingRaw != "" {

		for _, rawMap := range strings.Split(channelsMappingRaw, ";") {
			splittedMap := strings.Split(rawMap, ":")

			if len(splittedMap) != 2 {
				return nil, fmt.Errorf("Wrong channels mapping: %v\n", rawMap)
			}

			cubeChannel := splittedMap[0]
			handlerChannel := splittedMap[1]

			channelsMapping[handlerChannel] = cubeChannel
		}
	}

	return &channelsMapping, nil
}

func parsePortsMapping(portsMappingRaw string) (*[]instance.PortMap, error) {

	var portsMapping []instance.PortMap
	if portsMappingRaw != "" {

		for _, rawMap := range strings.Split(portsMappingRaw, ";") {
			splittedMap := strings.Split(rawMap, ":")

			if len(splittedMap) != 3 {
				return nil, fmt.Errorf("Wrong ports mapping: %v\n", rawMap)
			}

			hostPort, err := strconv.ParseUint(splittedMap[0], 10, 32)
			if err != nil {
				return nil, fmt.Errorf("Wrong host port format: %v/n", hostPort)
			}

			handlerPort, err := strconv.ParseUint(splittedMap[1], 10, 32)
			if err != nil {
				return nil, fmt.Errorf("Wrong cube port format: %v/n", handlerPort)
			}

			protocol := splittedMap[2]
			if protocol != "udp" && protocol != "tcp" {
				return nil, fmt.Errorf("Wrong port protocol: %v/n", protocol)
			}

			portsMapping = append(portsMapping, instance.PortMap{
				HostPort: uint(hostPort),
				CubePort: uint(handlerPort),
				Protocol: protocol,
			})
		}
	}

	return &portsMapping, nil
}

func instanceAdd(c *cli.Context) error {
	args := c.Args()

	//TODO: add instance name format check
	name := args.Get(0)
	if name == "" {
		return fmt.Errorf("instance name is required")
	}

	source := args.Get(1)
	if source == "" {
		return fmt.Errorf("instance source is required")
	}

	channelsMappingRaw := c.String("channels")
	channelsMapping, err := parseChannelsMapping(channelsMappingRaw)
	if err != nil {
		return err
	}

	portsMappingRaw := c.String("ports")
	portsMapping, err := parsePortsMapping(portsMappingRaw)
	if err != nil {
		return err
	}

	err = instance.Add(
		name,
		source,
		map[string]string{},
		*portsMapping,
		*channelsMapping,
	)

	return err
}

func instanceRemove(c *cli.Context) error {
	args := c.Args()

	name := args.Get(0)
	if name == "" {
		return fmt.Errorf("instance name is required")
	}

	return instance.Remove(name)
}

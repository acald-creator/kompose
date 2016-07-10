/*
Copyright 2016 Skippbox, Ltd All rights reserved.

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

package app

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/urfave/cli"

	"github.com/docker/libcompose/project"

	"encoding/json"
	"io/ioutil"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/extensions"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/util/intstr"

	"github.com/fatih/structs"
	"github.com/ghodss/yaml"
)

type ProjectAction func(project *project.Project, c *cli.Context)

const letterBytes = "abcdefghijklmnopqrstuvwxyz0123456789"

var unsupportedKey = map[string]string{
	"Build":         "",
	"CapAdd":        "",
	"CapDrop":       "",
	"CPUSet":        "",
	"CPUShares":     "",
	"ContainerName": "",
	"Devices":       "",
	"DNS":           "",
	"DNSSearch":     "",
	"Dockerfile":    "",
	"DomainName":    "",
	"Entrypoint":    "",
	"EnvFile":       "",
	"Hostname":      "",
	"LogDriver":     "",
	"MemLimit":      "",
	"MemSwapLimit":  "",
	"Net":           "",
	"Pid":           "",
	"Uts":           "",
	"Ipc":           "",
	"ReadOnly":      "",
	"StdinOpen":     "",
	"SecurityOpt":   "",
	"Tty":           "",
	"User":          "",
	"VolumeDriver":  "",
	"VolumesFrom":   "",
	"Expose":        "",
	"ExternalLinks": "",
	"LogOpt":        "",
	"ExtraHosts":    "",
}

// RandStringBytes generates randomly n-character string
func RandStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

// BeforeApp is an action that is executed before any cli command.
func BeforeApp(c *cli.Context) error {
	if c.GlobalBool("verbose") {
		logrus.SetLevel(logrus.DebugLevel)
	}
	// logrus.Warning("Note: This is an experimental alternate implementation of the Docker Compose CLI (https://github.com/docker/compose)")
	return nil
}

// WithProject is an helper function to create a cli.Command action with a ProjectFactory.
func WithProject(factory ProjectFactory, action ProjectAction) func(context *cli.Context) {
	return func(context *cli.Context) {
		p, err := factory.Create(context)
		if err != nil {
			logrus.Fatalf("Failed to read project: %v", err)
		}
		action(p, context)
	}
}

// ProjectKuberPS lists all rc, svc.
func ProjectKuberPS(p *project.Project, c *cli.Context) {
	factory := cmdutil.NewFactory(nil)
	clientConfig, err := factory.ClientConfig()
	if err != nil {
		logrus.Fatalf("Failed to get Kubernetes client config: %v", err)
	}
	client := client.NewOrDie(clientConfig)

	if c.BoolT("svc") {
		fmt.Printf("%-20s%-20s%-20s%-20s\n", "Name", "Cluster IP", "Ports", "Selectors")
		for name := range p.Configs {
			var ports string
			var selectors string
			services, err := client.Services(api.NamespaceDefault).Get(name)

			if err != nil {
				logrus.Debugf("Cannot find service for: ", name)
			} else {

				for i := range services.Spec.Ports {
					p := strconv.Itoa(int(services.Spec.Ports[i].Port))
					ports += ports + string(services.Spec.Ports[i].Protocol) + "(" + p + "),"
				}

				for k, v := range services.ObjectMeta.Labels {
					selectors += selectors + k + "=" + v + ","
				}

				ports = strings.TrimSuffix(ports, ",")
				selectors = strings.TrimSuffix(selectors, ",")

				fmt.Printf("%-20s%-20s%-20s%-20s\n", services.ObjectMeta.Name,
					services.Spec.ClusterIP, ports, selectors)
			}

		}
	}

	if c.BoolT("rc") {
		fmt.Printf("%-15s%-15s%-30s%-10s%-20s\n", "Name", "Containers", "Images",
			"Replicas", "Selectors")
		for name := range p.Configs {
			var selectors string
			var containers string
			var images string
			rc, err := client.ReplicationControllers(api.NamespaceDefault).Get(name)

			/* Should grab controller, container, image, selector, replicas */

			if err != nil {
				logrus.Debugf("Cannot find rc for: ", string(name))
			} else {

				for k, v := range rc.Spec.Selector {
					selectors += selectors + k + "=" + v + ","
				}

				for i := range rc.Spec.Template.Spec.Containers {
					c := rc.Spec.Template.Spec.Containers[i]
					containers += containers + c.Name + ","
					images += images + c.Image + ","
				}
				selectors = strings.TrimSuffix(selectors, ",")
				containers = strings.TrimSuffix(containers, ",")
				images = strings.TrimSuffix(images, ",")

				fmt.Printf("%-15s%-15s%-30s%-10d%-20s\n", rc.ObjectMeta.Name, containers,
					images, rc.Spec.Replicas, selectors)
			}
		}
	}

}

// ProjectKuberDelete deletes all rc, svc.
func ProjectKuberDelete(p *project.Project, c *cli.Context) {
	factory := cmdutil.NewFactory(nil)
	clientConfig, err := factory.ClientConfig()
	if err != nil {
		logrus.Fatalf("Failed to get Kubernetes client config: %v", err)
	}
	client := client.NewOrDie(clientConfig)

	for name := range p.Configs {
		if len(c.String("name")) > 0 && name != c.String("name") {
			continue
		}

		if c.BoolT("svc") {
			err := client.Services(api.NamespaceDefault).Delete(name)
			if err != nil {
				logrus.Fatalf("Unable to delete service %s: %s\n", name, err)
			}
		} else if c.BoolT("rc") {
			err := client.ReplicationControllers(api.NamespaceDefault).Delete(name)
			if err != nil {
				logrus.Fatalf("Unable to delete replication controller %s: %s\n", name, err)
			}
		}
	}
}

// ProjectKuberScale scales rc.
func ProjectKuberScale(p *project.Project, c *cli.Context) {
	factory := cmdutil.NewFactory(nil)
	clientConfig, err := factory.ClientConfig()
	if err != nil {
		logrus.Fatalf("Failed to get Kubernetes client config: %v", err)
	}
	client := client.NewOrDie(clientConfig)

	if c.Int("scale") <= 0 {
		logrus.Fatalf("Scale must be defined and a positive number")
	}

	for name := range p.Configs {
		if len(c.String("rc")) == 0 || c.String("rc") == name {
			s, err := client.ExtensionsClient.Scales(api.NamespaceDefault).Get("ReplicationController", name)
			if err != nil {
				logrus.Fatalf("Error retrieving scaling data: %s\n", err)
			}

			s.Spec.Replicas = int32(c.Int("scale"))

			s, err = client.ExtensionsClient.Scales(api.NamespaceDefault).Update("ReplicationController", s)
			if err != nil {
				logrus.Fatalf("Error updating scaling data: %s\n", err)
			}

			fmt.Printf("Scaling %s to: %d\n", name, s.Spec.Replicas)
		}
	}
}

// Create the file to write to if --out is specified
func createOutFile(out string) *os.File {
	var f *os.File
	var err error
	if len(out) != 0 {
		f, err = os.Create(out)
		if err != nil {
			logrus.Fatalf("error opening file: %v", err)
		}
	}
	return f
}

// Init RC object
func initRC(name string, service *project.ServiceConfig) *api.ReplicationController {
	rc := &api.ReplicationController{
		TypeMeta: unversioned.TypeMeta{
			Kind:       "ReplicationController",
			APIVersion: "v1",
		},
		ObjectMeta: api.ObjectMeta{
			Name: name,
			//Labels: map[string]string{"service": name},
		},
		Spec: api.ReplicationControllerSpec{
			Replicas: 1,
			Selector: map[string]string{"service": name},
			Template: &api.PodTemplateSpec{
				ObjectMeta: api.ObjectMeta{
				//Labels: map[string]string{"service": name},
				},
				Spec: api.PodSpec{
					Containers: []api.Container{
						{
							Name:  name,
							Image: service.Image,
						},
					},
				},
			},
		},
	}
	return rc
}

// Init SC object
func initSC(name string, service *project.ServiceConfig) *api.Service {
	sc := &api.Service{
		TypeMeta: unversioned.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: api.ObjectMeta{
			Name: name,
			//Labels: map[string]string{"service": name},
		},
		Spec: api.ServiceSpec{
			Selector: map[string]string{"service": name},
		},
	}
	return sc
}

// Init DC object
func initDC(name string, service *project.ServiceConfig) *extensions.Deployment {
	dc := &extensions.Deployment{
		TypeMeta: unversioned.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "extensions/v1beta1",
		},
		ObjectMeta: api.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"service": name},
		},
		Spec: extensions.DeploymentSpec{
			Replicas: 1,
			Selector: &unversioned.LabelSelector{
				MatchLabels: map[string]string{"service": name},
			},
			//UniqueLabelKey: p.Name,
			Template: api.PodTemplateSpec{
				ObjectMeta: api.ObjectMeta{
					Labels: map[string]string{"service": name},
				},
				Spec: api.PodSpec{
					Containers: []api.Container{
						{
							Name:  name,
							Image: service.Image,
						},
					},
				},
			},
		},
	}
	return dc
}

// Init DS object
func initDS(name string, service *project.ServiceConfig) *extensions.DaemonSet {
	ds := &extensions.DaemonSet{
		TypeMeta: unversioned.TypeMeta{
			Kind:       "DaemonSet",
			APIVersion: "extensions/v1beta1",
		},
		ObjectMeta: api.ObjectMeta{
			Name: name,
		},
		Spec: extensions.DaemonSetSpec{
			Template: api.PodTemplateSpec{
				ObjectMeta: api.ObjectMeta{
					Name: name,
				},
				Spec: api.PodSpec{
					Containers: []api.Container{
						{
							Name:  name,
							Image: service.Image,
						},
					},
				},
			},
		},
	}
	return ds
}

// Init RS object
func initRS(name string, service *project.ServiceConfig) *extensions.ReplicaSet {
	rs := &extensions.ReplicaSet{
		TypeMeta: unversioned.TypeMeta{
			Kind:       "ReplicaSet",
			APIVersion: "extensions/v1beta1",
		},
		ObjectMeta: api.ObjectMeta{
			Name: name,
		},
		Spec: extensions.ReplicaSetSpec{
			Replicas: 1,
			Selector: &unversioned.LabelSelector{
				MatchLabels: map[string]string{"service": name},
			},
			Template: api.PodTemplateSpec{
				ObjectMeta: api.ObjectMeta{},
				Spec: api.PodSpec{
					Containers: []api.Container{
						{
							Name:  name,
							Image: service.Image,
						},
					},
				},
			},
		},
	}
	return rs
}

// Configure the environment variables.
func configEnvs(name string, service *project.ServiceConfig) ([]api.EnvVar, string) {
	var envs []api.EnvVar
	for _, env := range service.Environment.Slice() {
		var character string = "="
		if strings.Contains(env, character) {
			value := env[strings.Index(env, character)+1:]
			name := env[0:strings.Index(env, character)]
			name = strings.TrimSpace(name)
			value = strings.TrimSpace(value)
			envs = append(envs, api.EnvVar{
				Name:  name,
				Value: value,
			})
		} else {
			character = ":"
			if strings.Contains(env, character) {
				var charQuote string = "'"
				value := env[strings.Index(env, character)+1:]
				name := env[0:strings.Index(env, character)]
				name = strings.TrimSpace(name)
				value = strings.TrimSpace(value)
				if strings.Contains(value, charQuote) {
					value = strings.Trim(value, "'")
				}
				envs = append(envs, api.EnvVar{
					Name:  name,
					Value: value,
				})
			} else {
				return nil, "Invalid container env " + env + " for service " + name
			}
		}
	}

	return envs, ""
}

// Configure the container volumes.
func configVolumes(service *project.ServiceConfig) ([]api.VolumeMount, []api.Volume) {
	var volumesMount []api.VolumeMount
	var volumes []api.Volume
	for _, volume := range service.Volumes {
		var character string = ":"
		if strings.Contains(volume, character) {
			hostDir := volume[0:strings.Index(volume, character)]
			hostDir = strings.TrimSpace(hostDir)
			containerDir := volume[strings.Index(volume, character)+1:]
			containerDir = strings.TrimSpace(containerDir)

			// check if ro/rw mode is defined
			var readonly bool = true
			if strings.Index(volume, character) != strings.LastIndex(volume, character) {
				mode := volume[strings.LastIndex(volume, character)+1:]
				if strings.Compare(mode, "rw") == 0 {
					readonly = false
				}
				containerDir = containerDir[0:strings.Index(containerDir, character)]
			}

			// volumeName = random string of 20 chars
			volumeName := RandStringBytes(20)

			volumesMount = append(volumesMount, api.VolumeMount{Name: volumeName, ReadOnly: readonly, MountPath: containerDir})
			p := &api.HostPathVolumeSource{
				Path: hostDir,
			}
			//p.Path = hostDir
			volumeSource := api.VolumeSource{HostPath: p}
			volumes = append(volumes, api.Volume{Name: volumeName, VolumeSource: volumeSource})
		}
	}
	return volumesMount, volumes
}

// Configure the container ports.
func configPorts(name string, service *project.ServiceConfig) ([]api.ContainerPort, string) {
	var ports []api.ContainerPort
	for _, port := range service.Ports {
		var character string = ":"
		if strings.Contains(port, character) {
			//portNumber := port[0:strings.Index(port, character)]
			targetPortNumber := port[strings.Index(port, character)+1:]
			targetPortNumber = strings.TrimSpace(targetPortNumber)
			targetPortNumberInt, err := strconv.Atoi(targetPortNumber)
			if err != nil {
				return nil, "Invalid container port " + port + " for service " + name
			}
			ports = append(ports, api.ContainerPort{ContainerPort: int32(targetPortNumberInt)})
		} else {
			portNumber, err := strconv.Atoi(port)
			if err != nil {
				return nil, "Invalid container port " + port + " for service " + name
			}
			ports = append(ports, api.ContainerPort{ContainerPort: int32(portNumber)})
		}
	}
	return ports, ""
}

// Configure the container service ports.
func configServicePorts(name string, service *project.ServiceConfig) ([]api.ServicePort, string) {
	var servicePorts []api.ServicePort
	for _, port := range service.Ports {
		var character string = ":"
		if strings.Contains(port, character) {
			portNumber := port[0:strings.Index(port, character)]
			portNumber = strings.TrimSpace(portNumber)
			targetPortNumber := port[strings.Index(port, character)+1:]
			targetPortNumber = strings.TrimSpace(targetPortNumber)
			portNumberInt, err := strconv.Atoi(portNumber)
			if err != nil {
				return nil, "Invalid container port " + port + " for service " + name
			}
			targetPortNumberInt, err1 := strconv.Atoi(targetPortNumber)
			if err1 != nil {
				return nil, "Invalid container port " + port + " for service " + name
			}
			var targetPort intstr.IntOrString
			targetPort.StrVal = targetPortNumber
			targetPort.IntVal = int32(targetPortNumberInt)
			servicePorts = append(servicePorts, api.ServicePort{Port: int32(portNumberInt), Name: portNumber, Protocol: "TCP", TargetPort: targetPort})
		} else {
			portNumber, err := strconv.Atoi(port)
			if err != nil {
				return nil, "Invalid container port " + port + " for service " + name
			}
			var targetPort intstr.IntOrString
			targetPort.StrVal = strconv.Itoa(portNumber)
			targetPort.IntVal = int32(portNumber)
			servicePorts = append(servicePorts, api.ServicePort{Port: int32(portNumber), Name: strconv.Itoa(portNumber), Protocol: "TCP", TargetPort: targetPort})
		}
	}
	return servicePorts, ""
}

// Transform data to json/yaml
func transformer(v interface{}, entity string, generateYaml bool) ([]byte, string) {
	// convert data to json / yaml
	data, err := json.MarshalIndent(v, "", "  ")
	if generateYaml == true {
		data, err = yaml.Marshal(v)
	}
	if err != nil {
		return nil, "Failed to marshal the " + entity
	}
	logrus.Debugf("%s\n", data)
	return data, ""
}

// ProjectKuberConvert tranforms docker compose to k8s objects
func ProjectKuberConvert(p *project.Project, c *cli.Context) {
	composeFile := c.String("file")
	outFile := c.String("out")
	generateYaml := c.BoolT("yaml")
	toStdout := c.BoolT("stdout")
	createD := c.BoolT("deployment")
	createDS := c.BoolT("daemonset")
	createRS := c.BoolT("replicaset")
	createChart := c.BoolT("chart")
	singleOutput := len(outFile) != 0 || toStdout

	// Validate the flags
	if len(outFile) != 0 && toStdout {
		logrus.Fatalf("Error: --out and --stdout can't be set at the same time")
	}
	if createChart && toStdout {
		logrus.Fatalf("Error: chart cannot be generated when --stdout is specified")
	}
	if singleOutput {
		count := 0
		if createD {
			count++
		}
		if createDS {
			count++
		}
		if createRS {
			count++
		}
		if count > 1 {
			logrus.Fatalf("Error: only one type of Kubernetes controller can be generated when --out or --stdout is specified")
		}
	}

	p = project.NewProject(&project.Context{
		ProjectName: "kube",
		ComposeFile: composeFile,
	})

	if err := p.Parse(); err != nil {
		logrus.Fatalf("Failed to parse the compose project from %s: %v", composeFile, err)
	}

	var f *os.File
	if !createChart {
		f = createOutFile(outFile)
		defer f.Close()
	}

	var mServices map[string][]byte = make(map[string][]byte)
	var mReplicationControllers map[string][]byte = make(map[string][]byte)
	var mDeployments map[string][]byte = make(map[string][]byte)
	var mDaemonSets map[string][]byte = make(map[string][]byte)
	var mReplicaSets map[string][]byte = make(map[string][]byte)
	var serviceLinks []string
	var svcnames []string

	for name, service := range p.Configs {
		svcnames = append(svcnames, name)

		checkUnsupportedKey(*service)

		rc := initRC(name, service)
		sc := initSC(name, service)
		dc := initDC(name, service)
		ds := initDS(name, service)
		rs := initRS(name, service)

		// Configure the environment variables.
		envs, err := configEnvs(name, service)
		if err != "" {
			logrus.Fatalf(err)
		}

		rc.Spec.Template.Spec.Containers[0].Env = envs
		dc.Spec.Template.Spec.Containers[0].Env = envs
		ds.Spec.Template.Spec.Containers[0].Env = envs
		rs.Spec.Template.Spec.Containers[0].Env = envs

		// Configure the container command.
		var cmds []string
		for _, cmd := range service.Command.Slice() {
			cmds = append(cmds, cmd)
		}
		rc.Spec.Template.Spec.Containers[0].Command = cmds
		dc.Spec.Template.Spec.Containers[0].Command = cmds
		ds.Spec.Template.Spec.Containers[0].Command = cmds
		rs.Spec.Template.Spec.Containers[0].Command = cmds

		// Configure the container working dir.
		rc.Spec.Template.Spec.Containers[0].WorkingDir = service.WorkingDir
		dc.Spec.Template.Spec.Containers[0].WorkingDir = service.WorkingDir
		ds.Spec.Template.Spec.Containers[0].WorkingDir = service.WorkingDir
		rs.Spec.Template.Spec.Containers[0].WorkingDir = service.WorkingDir

		// Configure the container volumes.
		volumesMount, volumes := configVolumes(service)

		rc.Spec.Template.Spec.Containers[0].VolumeMounts = volumesMount
		dc.Spec.Template.Spec.Containers[0].VolumeMounts = volumesMount
		ds.Spec.Template.Spec.Containers[0].VolumeMounts = volumesMount
		rs.Spec.Template.Spec.Containers[0].VolumeMounts = volumesMount

		rc.Spec.Template.Spec.Volumes = volumes
		dc.Spec.Template.Spec.Volumes = volumes
		ds.Spec.Template.Spec.Volumes = volumes
		rs.Spec.Template.Spec.Volumes = volumes

		// Configure the container privileged mode
		if service.Privileged == true {
			securitycontexts := &api.SecurityContext{
				Privileged: &service.Privileged,
			}
			rc.Spec.Template.Spec.Containers[0].SecurityContext = securitycontexts
			dc.Spec.Template.Spec.Containers[0].SecurityContext = securitycontexts
			ds.Spec.Template.Spec.Containers[0].SecurityContext = securitycontexts
			rs.Spec.Template.Spec.Containers[0].SecurityContext = securitycontexts
		}

		// Configure the container ports.
		ports, err := configPorts(name, service)
		if err != "" {
			logrus.Fatalf(err)
		}

		rc.Spec.Template.Spec.Containers[0].Ports = ports
		dc.Spec.Template.Spec.Containers[0].Ports = ports
		ds.Spec.Template.Spec.Containers[0].Ports = ports
		rs.Spec.Template.Spec.Containers[0].Ports = ports

		// Configure the service ports.
		servicePorts, err := configServicePorts(name, service)
		if err != "" {
			logrus.Fatalf(err)
		}

		sc.Spec.Ports = servicePorts

		// Configure label
		labels := map[string]string{"service": name}
		for key, value := range service.Labels.MapParts() {
			labels[key] = value
		}
		rc.Spec.Template.ObjectMeta.Labels = labels
		dc.Spec.Template.ObjectMeta.Labels = labels
		ds.Spec.Template.ObjectMeta.Labels = labels
		rs.Spec.Template.ObjectMeta.Labels = labels

		rc.ObjectMeta.Labels = labels
		dc.ObjectMeta.Labels = labels
		ds.ObjectMeta.Labels = labels
		rs.ObjectMeta.Labels = labels
		sc.ObjectMeta.Labels = labels

		// Configure the container restart policy.
		switch service.Restart {
		case "", "always":
			rc.Spec.Template.Spec.RestartPolicy = api.RestartPolicyAlways
			dc.Spec.Template.Spec.RestartPolicy = api.RestartPolicyAlways
			ds.Spec.Template.Spec.RestartPolicy = api.RestartPolicyAlways
			rs.Spec.Template.Spec.RestartPolicy = api.RestartPolicyAlways
		case "no":
			rc.Spec.Template.Spec.RestartPolicy = api.RestartPolicyNever
			dc.Spec.Template.Spec.RestartPolicy = api.RestartPolicyNever
			ds.Spec.Template.Spec.RestartPolicy = api.RestartPolicyNever
			rs.Spec.Template.Spec.RestartPolicy = api.RestartPolicyNever
		case "on-failure":
			rc.Spec.Template.Spec.RestartPolicy = api.RestartPolicyOnFailure
			dc.Spec.Template.Spec.RestartPolicy = api.RestartPolicyOnFailure
			ds.Spec.Template.Spec.RestartPolicy = api.RestartPolicyOnFailure
			rs.Spec.Template.Spec.RestartPolicy = api.RestartPolicyOnFailure
		default:
			logrus.Fatalf("Unknown restart policy %s for service %s", service.Restart, name)
		}

		// convert datarc to json / yaml
		datarc, err := transformer(rc, "replication controller", generateYaml)
		if err != "" {
			logrus.Fatalf(err)
		}

		// convert datadc to json / yaml
		datadc, err := transformer(dc, "deployment", generateYaml)
		if err != "" {
			logrus.Fatalf(err)
		}

		// convert datads to json / yaml
		datads, err := transformer(ds, "daemonSet", generateYaml)
		if err != "" {
			logrus.Fatalf(err)
		}

		// convert datars to json / yaml
		datars, err := transformer(rs, "replicaSet", generateYaml)
		if err != "" {
			logrus.Fatalf(err)
		}

		// convert datasvc to json / yaml
		datasvc, err := transformer(sc, "service controller", generateYaml)
		if err != "" {
			logrus.Fatalf(err)
		}

		mServices[name] = datasvc
		mReplicationControllers[name] = datarc
		mDeployments[name] = datadc
		mDaemonSets[name] = datads
		mReplicaSets[name] = datars
		exists := false

		if len(service.Links.Slice()) > 0 {
			for i := 0; i < len(service.Links.Slice()); i++ {
				var data string = service.Links.Slice()[i]
				for _, v := range serviceLinks {
					if v == data {
						exists = true
						break
					}
				}
				if !exists {
					serviceLinks = append(serviceLinks, data)
				}
			}
		}
	}

	for _, serviceLink := range serviceLinks {
		mServices[serviceLink] = nil
	}

	for k, v := range mServices {
		if v != nil {
			print(k, "svc", v, toStdout, generateYaml, f)
		}
	}

	// If --out or --stdout is set, the validation should already prevent multiple controllers being generated
	if createD {
		for k, v := range mDeployments {
			print(k, "deployment", v, toStdout, generateYaml, f)
		}
	}

	if createDS {
		for k, v := range mDaemonSets {
			print(k, "daemonset", v, toStdout, generateYaml, f)
		}
	}

	if createRS {
		for k, v := range mReplicaSets {
			print(k, "replicaset", v, toStdout, generateYaml, f)
		}
	}

	// We can create RC when we either don't print to --out or --stdout, or we don't create any other controllers
	if !singleOutput || (!createD && !createDS && !createRS) {
		for k, v := range mReplicationControllers {
			print(k, "rc", v, toStdout, generateYaml, f)
		}
	}

	if f != nil {
		fmt.Fprintf(os.Stdout, "file %q created\n", outFile)
	}

	if createChart {
		err := generateHelm(composeFile, svcnames, generateYaml)
		if err != nil {
			logrus.Fatalf("Failed to create Chart data: %s\n", err)
		}
	}
}

func checkUnsupportedKey(service project.ServiceConfig) {
	s := structs.New(service)
	for _, f := range s.Fields() {
		if f.IsExported() && !f.IsZero() {
			if _, ok := unsupportedKey[f.Name()]; ok {
				fmt.Println("WARNING: Unsupported key " + f.Name() + " - ignoring")
			}
		}
	}
}

func print(name, trailing string, data []byte, toStdout, generateYaml bool, f *os.File) {
	file := fmt.Sprintf("%s-%s.json", name, trailing)
	if generateYaml {
		file = fmt.Sprintf("%s-%s.yaml", name, trailing)
	}
	separator := ""
	if generateYaml {
		separator = "---"
	}
	if toStdout {
		fmt.Fprintf(os.Stdout, "%s%s\n", string(data), separator)
	} else if f != nil {
		// Write all content to a single file f
		if _, err := f.WriteString(fmt.Sprintf("%s%s\n", string(data), separator)); err != nil {
			logrus.Fatalf("Failed to write %s to file: %v", trailing, err)
		}
		f.Sync()
	} else {
		// Write content separately to each file
		if err := ioutil.WriteFile(file, []byte(data), 0644); err != nil {
			logrus.Fatalf("Failed to write %s: %v", trailing, err)
		}
		fmt.Fprintf(os.Stdout, "file %q created\n", file)
	}
}

// ProjectKuberUp brings up rc, svc.
func ProjectKuberUp(p *project.Project, c *cli.Context) {
	factory := cmdutil.NewFactory(nil)
	clientConfig, err := factory.ClientConfig()
	if err != nil {
		logrus.Fatalf("Failed to get Kubernetes client config: %v", err)
	}
	client := client.NewOrDie(clientConfig)

	files, err := ioutil.ReadDir(".")
	if err != nil {
		logrus.Fatalf("Failed to load rc, svc manifest files: %s\n", err)
	}

	// submit svc first
	sc := &api.Service{}
	for _, file := range files {
		if strings.Contains(file.Name(), "svc") {
			datasvc, err := ioutil.ReadFile(file.Name())

			if err != nil {
				logrus.Fatalf("Failed to load %s: %s\n", file.Name(), err)
			}

			if strings.Contains(file.Name(), "json") {
				err := json.Unmarshal(datasvc, &sc)
				if err != nil {
					logrus.Fatalf("Failed to unmarshal file %s to svc object: %s\n", file.Name(), err)
				}
			}
			if strings.Contains(file.Name(), "yaml") {
				err := yaml.Unmarshal(datasvc, &sc)
				if err != nil {
					logrus.Fatalf("Failed to unmarshal file %s to svc object: %s\n", file.Name(), err)
				}
			}
			// submit sc to k8s
			scCreated, err := client.Services(api.NamespaceDefault).Create(sc)
			if err != nil {
				fmt.Println(err)
			}
			logrus.Debugf("%s\n", scCreated)
		}
	}

	// then submit rc
	rc := &api.ReplicationController{}
	for _, file := range files {
		if strings.Contains(file.Name(), "rc") {
			datarc, err := ioutil.ReadFile(file.Name())

			if err != nil {
				logrus.Fatalf("Failed to load %s: %s\n", file.Name(), err)
			}

			if strings.Contains(file.Name(), "json") {
				err := json.Unmarshal(datarc, &rc)
				if err != nil {
					logrus.Fatalf("Failed to unmarshal file %s to rc object: %s\n", file.Name(), err)
				}
			}
			if strings.Contains(file.Name(), "yaml") {
				err := yaml.Unmarshal(datarc, &rc)
				if err != nil {
					logrus.Fatalf("Failed to unmarshal file %s to rc object: %s\n", file.Name(), err)
				}
			}
			// submit rc to k8s
			rcCreated, err := client.ReplicationControllers(api.NamespaceDefault).Create(rc)
			if err != nil {
				fmt.Println(err)
			}
			logrus.Debugf("%s\n", rcCreated)
		}
	}

}
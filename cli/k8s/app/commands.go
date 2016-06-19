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
    "strconv"
    "strings"
    "math/rand"

    "github.com/Sirupsen/logrus"
    "github.com/codegangsta/cli"

    "github.com/docker/libcompose/project"
    //project "github.com/skippbox/kompose/project"

    "encoding/json"
    "io/ioutil"

    "k8s.io/kubernetes/pkg/api"
    "k8s.io/kubernetes/pkg/apis/extensions"
    "k8s.io/kubernetes/pkg/util/intstr"
    "k8s.io/kubernetes/pkg/api/unversioned"
    client "k8s.io/kubernetes/pkg/client/unversioned"
    cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

    "github.com/ghodss/yaml"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyz0123456789"

func RandStringBytes(n int) string {
    b := make([]byte, n)
    for i := range b {
        b[i] = letterBytes[rand.Intn(len(letterBytes))]
    }
    return string(b)
}

func ProjectKuberPS(p *project.Project, c *cli.Context) {
    //server := getK8sServer("")

    factory := cmdutil.NewFactory(nil)
    clientConfig, err := factory.ClientConfig()
    if err != nil {
        logrus.Fatalf("Failed to get Kubernetes client config: %v", err)
    }
    client := client.NewOrDie(clientConfig)

    if c.BoolT("svc") {
        fmt.Printf("%-20s%-20s%-20s%-20s\n","Name", "Cluster IP", "Ports", "Selectors")
        for name := range p.Configs {
            var ports string
            var selectors string
            services, err := client.Services(api.NamespaceDefault).Get(name)

            if err != nil {
                logrus.Debugf("Cannot find service for: ", name)
            } else {

                for i := range services.Spec.Ports {
                    p := strconv.Itoa(services.Spec.Ports[i].Port)
                    ports += ports + string(services.Spec.Ports[i].Protocol) + "(" + p + "),"
                }

                for k,v := range services.ObjectMeta.Labels {
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

                for k,v := range rc.Spec.Selector {
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

func ProjectKuberDelete(p *project.Project, c *cli.Context) {
    //server := getK8sServer("")

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

func ProjectKuberScale(p *project.Project, c *cli.Context) {
    //server := getK8sServer("")

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

            s.Spec.Replicas = c.Int("scale")

            s, err = client.ExtensionsClient.Scales(api.NamespaceDefault).Update("ReplicationController", s)
            if err != nil {
                logrus.Fatalf("Error updating scaling data: %s\n", err)
            }

            fmt.Printf("Scaling %s to: %d\n", name, s.Spec.Replicas)
        }
    }
}

func ProjectKuberConvert(p *project.Project, c *cli.Context) {
    //createInstance := true
    generateYaml := false
    composeFile := c.String("file")

    p = project.NewProject(&project.Context{
        ProjectName: "kube",
        ComposeFile: composeFile,
    })

    if err := p.Parse(); err != nil {
        logrus.Fatalf("Failed to parse the compose project from %s: %v", composeFile, err)
    }

    //server := getK8sServer("")

    //if c.BoolT("deployment") || c.BoolT("chart") || c.BoolT("daemonset") || c.BoolT("replicaset") {
    //    createInstance = false
    //}

    if c.BoolT("yaml") {
        generateYaml = true
    }

    var mServices map[string]api.Service = make(map[string]api.Service)
    var serviceLinks []string

    //version := "v1"
    // create new client
    //client := client.NewOrDie(&client.Config{Host: server, Version: version})
    //client := client.NewOrDie(&restclient.Config{Host: server})

    for name, service := range p.Configs {
        rc := &api.ReplicationController{
            TypeMeta: unversioned.TypeMeta{
                Kind:       "ReplicationController",
                APIVersion: "v1",
            },
            ObjectMeta: api.ObjectMeta{
                Name:   name,
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
        sc := &api.Service{
            TypeMeta: unversioned.TypeMeta{
                Kind:       "Service",
                APIVersion: "v1",
            },
            ObjectMeta: api.ObjectMeta{
                Name:   name,
                //Labels: map[string]string{"service": name},
            },
            Spec: api.ServiceSpec{
                Selector: map[string]string{"service": name},
            },
        }
        dc := &extensions.Deployment {
            TypeMeta: unversioned.TypeMeta {
                Kind:       "Deployment",
                APIVersion: "extensions/v1beta1",
            },
            ObjectMeta: api.ObjectMeta{
                Name: name,
                Labels: map[string]string{"service": name},
            },
            Spec: extensions.DeploymentSpec {
                Replicas: 1,
                Selector: &unversioned.LabelSelector{
                    MatchLabels: map[string]string{"service": name},
                },
                //UniqueLabelKey: p.Name,
                Template: api.PodTemplateSpec {
                    ObjectMeta: api.ObjectMeta {
                        Labels: map[string]string {"service": name},
                    },
                    Spec: api.PodSpec {
                        Containers: []api.Container {
                            {
                                Name:  name,
                                Image: service.Image,
                            },
                        },
                    },
                },
            },
        }
        ds := &extensions.DaemonSet{
            TypeMeta: unversioned.TypeMeta{
                Kind: "DaemonSet",
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
                    Spec: api.PodSpec {
                        Containers: []api.Container {
                            {
                                Name:  name,
                                Image: service.Image,
                            },
                        },
                    },
                },
            },
        }
        rs := &extensions.ReplicaSet{
            TypeMeta: unversioned.TypeMeta{
                Kind: "ReplicaSet",
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
                    ObjectMeta: api.ObjectMeta{

                    },
                    Spec: api.PodSpec{
                        Containers: []api.Container{
                            {
                                Name: name,
                                Image: service.Image,
                            },
                        },
                    },
                },
            },
        }

        // Configure the environment variables.
        var envs []api.EnvVar
        for _, env := range service.Environment.Slice() {
            var character string = "="
            if strings.Contains(env, character) {
                value := env[strings.Index(env, character) + 1: len(env)]
                name :=  env[0:strings.Index(env, character)]
                name = strings.TrimSpace(name)
                value = strings.TrimSpace(value)
                envs = append(envs, api.EnvVar{
                    Name: name,
                    Value: value,
                })
            } else {
                character = ":"
                if strings.Contains(env, character) {
                    var charQuote string = "'"
                    value := env[strings.Index(env, character) + 1: len(env)]
                    name := env[0:strings.Index(env, character)]
                    name = strings.TrimSpace(name)
                    value = strings.TrimSpace(value)
                    if strings.Contains(value, charQuote) {
                        value = strings.Trim(value, "'")
                    }
                    envs = append(envs, api.EnvVar{
                        Name: name,
                        Value: value,
                    })
                } else {
                    logrus.Fatalf("Invalid container env %s for service %s", env, name)
                }
            }
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
        var volumesMount []api.VolumeMount
        var volumes []api.Volume
        for _, volume := range service.Volumes {
            var character string = ":"
            if strings.Contains(volume, character) {
                hostDir := volume[0:strings.Index(volume, character)]
                hostDir = strings.TrimSpace(hostDir)
                containerDir := volume[strings.Index(volume, character) + 1: len(volume)]
                containerDir = strings.TrimSpace(containerDir)

                // check if ro/rw mode is defined
                var readonly bool = true
                if strings.Index(volume, character) != strings.LastIndex(volume, character) {
                    mode := volume[strings.LastIndex(volume, character) + 1: len(volume)]
                    if strings.Compare(mode, "rw") == 0 {
                        readonly = false
                    }
                    containerDir = containerDir[0:strings.Index(containerDir, character)]
                }

                // volumeName = random string of 20 chars
                volumeName := RandStringBytes(20)

                volumesMount = append(volumesMount, api.VolumeMount{Name: volumeName, ReadOnly: readonly, MountPath: containerDir})
                p := &api.HostPathVolumeSource {
                    Path: hostDir,
                }
                //p.Path = hostDir
                volumeSource := api.VolumeSource{HostPath: p}
                volumes = append(volumes, api.Volume{Name: volumeName, VolumeSource: volumeSource})
            }
        }

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
        var ports []api.ContainerPort
        for _, port := range service.Ports {
            var character string = ":"
            if strings.Contains(port, character) {
                //portNumber := port[0:strings.Index(port, character)]
                targetPortNumber := port[strings.Index(port, character) + 1: len(port)]
                targetPortNumber = strings.TrimSpace(targetPortNumber)
                targetPortNumberInt, err := strconv.Atoi(targetPortNumber)
                if err != nil {
                    logrus.Fatalf("Invalid container port %s for service %s", port, name)
                }
                ports = append(ports, api.ContainerPort{ContainerPort: targetPortNumberInt})
            } else {
                portNumber, err := strconv.Atoi(port)
                if err != nil {
                    logrus.Fatalf("Invalid container port %s for service %s", port, name)
                }
                ports = append(ports, api.ContainerPort{ContainerPort: portNumber})
            }
        }

        rc.Spec.Template.Spec.Containers[0].Ports = ports
        dc.Spec.Template.Spec.Containers[0].Ports = ports
        ds.Spec.Template.Spec.Containers[0].Ports = ports
        rs.Spec.Template.Spec.Containers[0].Ports = ports

        // Configure the service ports.
        var servicePorts []api.ServicePort
        for _, port := range service.Ports {
            var character string = ":"
            if strings.Contains(port, character) {
                portNumber := port[0:strings.Index(port, character)]
                portNumber = strings.TrimSpace(portNumber)
                targetPortNumber := port[strings.Index(port, character) + 1: len(port)]
                targetPortNumber = strings.TrimSpace(targetPortNumber)
                portNumberInt, err := strconv.Atoi(portNumber)
                if err != nil {
                    logrus.Fatalf("Invalid container port %s for service %s", port, name)
                }
                targetPortNumberInt, err1 := strconv.Atoi(targetPortNumber)
                if err1 != nil {
                    logrus.Fatalf("Invalid container port %s for service %s", port, name)
                }
                var targetPort intstr.IntOrString
                targetPort.StrVal = targetPortNumber
                targetPort.IntVal = int32(targetPortNumberInt)
                servicePorts = append(servicePorts, api.ServicePort{Port: portNumberInt, Name: portNumber, Protocol: "TCP", TargetPort: targetPort})
            } else {
                portNumber, err := strconv.Atoi(port)
                if err != nil {
                    logrus.Fatalf("Invalid container port %s for service %s", port, name)
                }
                var targetPort intstr.IntOrString
                targetPort.StrVal = strconv.Itoa(portNumber)
                targetPort.IntVal = int32(portNumber)
                servicePorts = append(servicePorts, api.ServicePort{Port: portNumber, Name: strconv.Itoa(portNumber), Protocol: "TCP", TargetPort: targetPort})
            }
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
        datarc, err := json.MarshalIndent(rc, "", "  ")
        if generateYaml == true {
            datarc, err = yaml.Marshal(rc)
        }

        if err != nil {
            logrus.Fatalf("Failed to marshal the replication controller: %v", err)
        }
        logrus.Debugf("%s\n", datarc)

        // convert datadc to json / yaml
        datadc, err := json.MarshalIndent(dc, "", "  ")
        if generateYaml == true {
            datadc, err = yaml.Marshal(dc)
        }

        if err != nil {
            logrus.Fatalf("Failed to marshal the deployment container: %v", err)
        }

        logrus.Debugf("%s\n", datadc)

        //convert datads to json / yaml
        datads, err := json.MarshalIndent(ds, "", "  ")
        if generateYaml == true {
            datads, err = yaml.Marshal(ds)
        }

        if err != nil {
            logrus.Fatalf("Failed to marshal the daemonSet: %v", err)
        }

        logrus.Debugf("%s\n", datads)

        //convert datars to json / yaml
        datars, err := json.MarshalIndent(rs, "", "  ")
        if generateYaml == true {
            datads, err = yaml.Marshal(rs)
        }

        if err != nil {
            logrus.Fatalf("Failed to marshal the replicaSet: %v", err)
        }

        logrus.Debugf("%s\n", datars)


        mServices[name] = *sc

        if len(service.Links.Slice()) > 0 {
            for i := 0; i < len(service.Links.Slice()); i++ {
                var data string = service.Links.Slice()[i]
                if len(serviceLinks) == 0 {
                    serviceLinks = append(serviceLinks, data)
                } else {
                    for _, v := range serviceLinks {
                        if v != data {
                            serviceLinks = append(serviceLinks, data)
                        }
                    }
                }
            }
        }

        // call create RC api
        //if createInstance == true {
        //    rcCreated, err := client.ReplicationControllers(api.NamespaceDefault).Create(rc)
        //    if err != nil {
        //        fmt.Println(err)
        //    }
        //    logrus.Debugf("%s\n", rcCreated)
        //}

        fileRC := fmt.Sprintf("%s-rc.json", name)
        if generateYaml == true {
            fileRC = fmt.Sprintf("%s-rc.yaml", name)
        }
        if err := ioutil.WriteFile(fileRC, []byte(datarc), 0644); err != nil {
            logrus.Fatalf("Failed to write replication controller: %v", err)
        }

        /* Create the deployment container */
        if c.BoolT("deployment") {
            fileDC := fmt.Sprintf("%s-deployment.json", name)
            if generateYaml == true {
                fileDC = fmt.Sprintf("%s-deployment.yaml", name)
            }
            if err := ioutil.WriteFile(fileDC, []byte(datadc), 0644); err != nil {
                logrus.Fatalf("Failed to write deployment container: %v", err)
            }
        }

        /* Create the daemonset container */
        if c.BoolT("daemonset") {
            fileDS := fmt.Sprintf("%s-daemonset.json", name)
            if generateYaml == true {
                fileDS = fmt.Sprintf("%s-daemonset.yaml", name)
            }
            if err := ioutil.WriteFile(fileDS, []byte(datads), 0644); err != nil {
                logrus.Fatalf("Failed to write daemonset: %v", err)
            }
        }

        /* Create the replicaset container */
        if c.BoolT("replicaset") {
            fileRS := fmt.Sprintf("%s-replicaset.json", name)
            if generateYaml == true {
                fileRS = fmt.Sprintf("%s-replicaset.yaml", name)
            }
            if err := ioutil.WriteFile(fileRS, []byte(datars), 0644); err != nil {
                logrus.Fatalf("Failed to write replicaset: %v", err)
            }
        }

        for k, v := range mServices {
            for i :=0; i < len(serviceLinks); i++ {
                //if serviceLinks[i] == k {
                // call create SVC api
                //if createInstance == true {
                //    scCreated, err := client.Services(api.NamespaceDefault).Create(&v)
                //    if err != nil {
                //        fmt.Println(err)
                //    }
                //    logrus.Debugf("%s\n", scCreated)
                //}


                // convert datasvc to json / yaml
                datasvc, er := json.MarshalIndent(v, "", "  ")
                if generateYaml == true {
                    datasvc, er = yaml.Marshal(v)
                }
                if er != nil {
                    logrus.Fatalf("Failed to marshal the service controller: %v", er)
                }

                logrus.Debugf("%s\n", datasvc)

                fileSVC := fmt.Sprintf("%s-svc.json", k)
                if generateYaml == true {
                    fileSVC = fmt.Sprintf("%s-svc.yaml", k)
                }

                if err := ioutil.WriteFile(fileSVC, []byte(datasvc), 0644); err != nil {
                    logrus.Fatalf("Failed to write service controller: %v", err)
                }
                //}
            }
        }
    }

    /* Need to iterate through one more time to ensure we capture all service/rc */
    for name := range p.Configs {
        if c.BoolT("chart") {
            err := generateHelm(composeFile, name)
            if err != nil {
                logrus.Fatalf("Failed to create Chart data: %s\n", err)
            }
        }
    }
}

func ProjectKuberUp(p *project.Project, c *cli.Context) {
    //server := getK8sServer("")

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
      // fmt.Println(file.Name())
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
        // fmt.Println(sc)
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
      // fmt.Println(file.Name())
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
        // fmt.Println(rc)
        rcCreated, err := client.ReplicationControllers(api.NamespaceDefault).Create(rc)
        if err != nil {
            fmt.Println(err)
        }
        logrus.Debugf("%s\n", rcCreated)
      }
    }

}


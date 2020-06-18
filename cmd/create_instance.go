/*
Copyright (c) SiteWhere, LLC. All rights reserved. http://www.sitewhere.com

The software in this package is published under the terms of the CPAL v1.0
license, a copy of which has been included with this distribution in the
LICENSE file.
*/

package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/sitewhere/swctl/internal"
	"github.com/sitewhere/swctl/pkg/apis/v1/alpha3"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	rbacV1 "k8s.io/api/rbac/v1"

	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// createInstanceCmd represents the instance command
var (
	namespace         = ""
	createInstanceCmd = &cobra.Command{
		Use:   "instance",
		Short: "Create SiteWhere Instance",
		Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires one argument")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]

			if namespace == "" {
				namespace = name
			}

			instance := alpha3.SiteWhereInstance{
				Name:                  name,
				Namespace:             namespace,
				ConfigurationTemplate: "default",
				DatasetTemplate:       "default"}

			createSiteWhereInstance(&instance)
		},
	}
)

func init() {
	createInstanceCmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace of the instance.")
	createCmd.AddCommand(createInstanceCmd)
}

func createSiteWhereInstance(instance *alpha3.SiteWhereInstance) {
	var err error

	config, err := internal.GetKubeConfigFromKubeconfig()
	if err != nil {
		fmt.Printf("Error getting Kubernetes Config: %v", err)
		return
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Printf("Error getting Kubernetes Client: %v", err)
		return
	}

	var ns *v1.Namespace
	ns, err = createNamespaceIfNotExists(instance.Namespace, clientset)
	if err != nil {
		fmt.Printf("Error Creating Namespace: %s, %v", instance.Namespace, err)
		return
	}

	var namespace = ns.ObjectMeta.Name

	var sa *v1.ServiceAccount
	sa, err = createServiceAccountIfNotExists(instance, namespace, clientset)
	if err != nil {
		fmt.Printf("Error Creating Service Account: %s, %v", instance.Namespace, err)
		return
	}

	var role *rbacV1.Role
	role, err = createRoleIfNotExists(instance, namespace, clientset)
	if err != nil {
		fmt.Printf("Error Creating Role: %s, %v", instance.Namespace, err)
		return
	}

	_, err = createRoleBindingIfNotExists(instance, namespace, sa, role, clientset)
	if err != nil {
		fmt.Printf("Error Creating Role Binding: %s, %v", instance.Namespace, err)
		return
	}

	client, err := dynamic.NewForConfig(config)
	if err != nil {
		fmt.Printf("Error getting Kubernetes Client: %v", err)
		return
	}

	_, err = createCRSiteWhereInstaceIfNotExists(instance, namespace, client)
	if err != nil {
		fmt.Printf("Error Creating CR SiteWhere Instace: %v", err)
		return
	}

	_, err = createCRSiteWhereAssetManagementIfNotExists(instance, namespace, client)
	if err != nil {
		fmt.Printf("Error Creating SiteWhere Asset Management Microservice: %v", err)
		return
	}

}

func createNamespaceIfNotExists(namespace string, clientset *kubernetes.Clientset) (*v1.Namespace, error) {
	var err error
	var ns *v1.Namespace

	ns, err = clientset.CoreV1().Namespaces().Get(context.TODO(), namespace, metav1.GetOptions{})

	if err != nil && k8serror.IsNotFound(err) {
		ns = &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
				Labels: map[string]string{
					"app": namespace,
				},
			},
		}

		result, err := clientset.CoreV1().Namespaces().Create(context.TODO(),
			ns,
			metav1.CreateOptions{})

		if err != nil {
			return nil, err
		}

		return result, err
	}

	if err != nil {
		return nil, err
	}

	return ns, nil
}

func createServiceAccountIfNotExists(instance *alpha3.SiteWhereInstance, namespace string, clientset *kubernetes.Clientset) (*v1.ServiceAccount, error) {
	var err error
	var sa *v1.ServiceAccount

	saName := fmt.Sprintf("sitewhere-instance-service-account-%s", namespace)

	sa, err = clientset.CoreV1().ServiceAccounts(namespace).Get(context.TODO(), saName, metav1.GetOptions{})

	if err != nil && k8serror.IsNotFound(err) {
		sa = &v1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name: saName,
				Labels: map[string]string{
					"app": instance.Name,
				},
			},
		}

		result, err := clientset.CoreV1().ServiceAccounts(namespace).Create(context.TODO(),
			sa,
			metav1.CreateOptions{})

		if err != nil {
			return nil, err
		}

		return result, err
	}

	if err != nil {
		return nil, err
	}

	return sa, nil
}

func createRoleIfNotExists(instance *alpha3.SiteWhereInstance, namespace string, clientset *kubernetes.Clientset) (*rbacV1.Role, error) {
	var err error
	var role *rbacV1.Role

	roleName := fmt.Sprintf("sitewhere-instance-role-%s", namespace)

	role, err = clientset.RbacV1().Roles(namespace).Get(context.TODO(), roleName, metav1.GetOptions{})
	if err != nil && k8serror.IsNotFound(err) {
		role = &rbacV1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name: roleName,
				Labels: map[string]string{
					"app": instance.Name,
				},
			},
			Rules: []rbacV1.PolicyRule{
				{
					APIGroups: []string{
						"sitewhere.io",
					},
					Resources: []string{
						"instances",
						"instances/status",
						"microservices",
						"tenants",
						"tenantengines",
						"tenantengines/status",
					},
					Verbs: []string{
						"*",
					},
				}, {
					APIGroups: []string{
						"templates.sitewhere.io",
					},
					Resources: []string{
						"instanceconfigurations",
						"instancedatasets",
						"tenantconfigurations",
						"tenantengineconfigurations",
						"tenantdatasets",
						"tenantenginedatasets",
					},
					Verbs: []string{
						"*",
					},
				}, {
					APIGroups: []string{
						"scripting.sitewhere.io",
					},
					Resources: []string{
						"scriptcategories",
						"scripttemplates",
						"scripts",
						"scriptversions",
					},
					Verbs: []string{
						"*",
					},
				}, {
					APIGroups: []string{
						"apiextensions.k8s.io",
					},
					Resources: []string{
						"customresourcedefinitions",
					},
					Verbs: []string{
						"*",
					},
				},
			},
		}

		result, err := clientset.RbacV1().Roles(namespace).Create(context.TODO(),
			role,
			metav1.CreateOptions{})

		if err != nil {
			return nil, err
		}

		return result, err
	}

	if err != nil {
		return nil, err
	}

	return role, nil
}

func createRoleBindingIfNotExists(instance *alpha3.SiteWhereInstance, namespace string, serviceAccount *v1.ServiceAccount,
	role *rbacV1.Role, clientset *kubernetes.Clientset) (*rbacV1.RoleBinding, error) {
	var err error
	var roleBinding *rbacV1.RoleBinding

	roleBindingName := fmt.Sprintf("sitewhere-instance-role-binding-%s", namespace)

	roleBinding, err = clientset.RbacV1().RoleBindings(namespace).Get(context.TODO(), roleBindingName, metav1.GetOptions{})
	if err != nil && k8serror.IsNotFound(err) {
		roleBinding = &rbacV1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: roleBindingName,
				Labels: map[string]string{
					"app": instance.Name,
				},
			},
			Subjects: []rbacV1.Subject{
				{
					Kind:      "ServiceAccount",
					Namespace: namespace,
					Name:      serviceAccount.ObjectMeta.Name,
				},
			},
			RoleRef: rbacV1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     role.ObjectMeta.Name,
			},
		}

		result, err := clientset.RbacV1().RoleBindings(namespace).Create(context.TODO(),
			roleBinding,
			metav1.CreateOptions{})

		if err != nil {
			return nil, err
		}

		return result, err
	}

	if err != nil {
		return nil, err
	}

	return roleBinding, nil
}

func createCRSiteWhereInstaceIfNotExists(instance *alpha3.SiteWhereInstance, namespace string, client dynamic.Interface) (*unstructured.Unstructured, error) {

	res := client.Resource(sitewhereInstanceGVR)

	sitewhereInstaces, err := res.Get(context.TODO(), instance.Name, metav1.GetOptions{})

	if err != nil && k8serror.IsNotFound(err) {
		sitewhereInstaces = &unstructured.Unstructured{
			Object: map[string]interface{}{
				"kind":       "SiteWhereInstance",
				"apiVersion": sitewhereInstanceGVR.Group + "/" + sitewhereInstanceGVR.Version,
				"metadata": map[string]interface{}{
					"name": instance.Name,
				},
				"spec": map[string]interface{}{
					"instanceNamespace":     instance.Namespace,
					"configurationTemplate": instance.ConfigurationTemplate,
					"datasetTemplate":       instance.DatasetTemplate,
				},
			},
		}

		result, err := res.Create(context.TODO(), sitewhereInstaces, metav1.CreateOptions{})
		if err != nil {
			return nil, err
		}

		return result, err
	}

	return nil, nil
}

func createCRSiteWhereAssetManagementIfNotExists(instance *alpha3.SiteWhereInstance, namespace string, client dynamic.Interface) (*unstructured.Unstructured, error) {

	res := client.Resource(sitewhereMicroserviceGVR).Namespace(namespace)

	assetManagementMS, err := res.Get(context.TODO(), "asset-management-microservice", metav1.GetOptions{})

	if err != nil && k8serror.IsNotFound(err) {
		assetManagementMS = &unstructured.Unstructured{
			Object: map[string]interface{}{
				"kind":       "SiteWhereMicroservice",
				"apiVersion": sitewhereMicroserviceGVR.Group + "/" + sitewhereMicroserviceGVR.Version,
				"metadata": map[string]interface{}{
					"name":      "asset-management-microservice",
					"namespace": namespace,
					"labels": map[string]interface{}{
						"sitewhere.io/instance":        instance.Name,
						"sitewhere.io/functional-area": "asset-management",
					},
				},
				"spec": map[string]interface{}{
					"replicas":    1, // TODO from parameter
					"multitenant": true,
					"name":        "Asset Management",
					"description": "Provides APIs for managing assets associated with device assignments",
					"icon":        "devices_other",
					"helm": map[string]interface{}{ // TODO Remove when operatior udpates to not using helm
						"chartName":      "sitewhere-0.3.0",
						"releaseName":    instance.Name,
						"releaseService": "Tiller",
					},
					"podSpec": map[string]interface{}{
						"imageRegistry":   "docker.io",
						"imageRepository": "sitewhere",
						"imageTag":        "3.0.0.beta1", // TODO from paramter
						"imagePullPolicy": "IfNotPresent",
						"ports": []map[string]interface{}{
							map[string]interface{}{
								"containerPort": 9000,
							},
							map[string]interface{}{
								"containerPort": 9090,
							},
						},
						"env": []map[string]interface{}{
							map[string]interface{}{
								"name": "sitewhere.config.k8s.name",
								"valueFrom": map[string]interface{}{
									"fieldRef": map[string]interface{}{
										"fieldPath": "metadata.name",
									},
								},
							},
							map[string]interface{}{
								"name": "sitewhere.config.k8s.namespace",
								"valueFrom": map[string]interface{}{
									"fieldRef": map[string]interface{}{
										"fieldPath": "metadata.namespace",
									},
								},
							},
							map[string]interface{}{
								"name": "sitewhere.config.k8s.pod.ip",
								"valueFrom": map[string]interface{}{
									"fieldRef": map[string]interface{}{
										"fieldPath": "status.podIP",
									},
								},
							},
						},
					},
					"serviceSpec": map[string]interface{}{
						"type": "ClusterIP",
						"ports": []map[string]interface{}{
							map[string]interface{}{
								"port":       9000,
								"targetPort": 9000,
								"name":       "grpc-api",
							},
							map[string]interface{}{
								"port":       9090,
								"targetPort": 9090,
								"name":       "http-metrics",
							},
						},
					},
					"debug": map[string]interface{}{
						"enabled":  false,
						"jdwpPort": 8006,
						"jmxPort":  1106,
					},
				},
			},
		}

		result, err := res.Create(context.TODO(), assetManagementMS, metav1.CreateOptions{})
		if err != nil {
			return nil, err
		}

		return result, err
	}

	return nil, nil

}

/*
Copyright (c) SiteWhere, LLC. All rights reserved. http://www.sitewhere.com

The software in this package is published under the terms of the CPAL v1.0
license, a copy of which has been included with this distribution in the
LICENSE file.
*/

// Package internal Implements swctl internal use only functions
package internal

import (
	"fmt"
	"net/http"

	"k8s.io/apimachinery/pkg/api/errors"

	"k8s.io/client-go/rest"
)

// Template for generating a Template Filename
const templateFileTemplate = "/templates/template-%02d.yaml"

// Number of CRD Files
const templatesFileNumber = 39

// InstallSiteWhereTemplates Install SiteWhere Templates CRD
func InstallSiteWhereTemplates(config *rest.Config, statikFS http.FileSystem) error {
	var err error
	for i := 1; i <= templatesFileNumber; i++ {
		var templateName = fmt.Sprintf(templateFileTemplate, i)
		CreateCustomResourceFromFile(templateName, config, statikFS)
		if err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	}
	return nil
}

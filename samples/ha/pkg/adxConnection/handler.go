/*
 * Copyright (c) Microsoft Corporation.
 * Licensed under the MIT License.
 */

package adxconnection

import (
	"os"

	"github.com/Azure/azure-kusto-go/kusto"
)

const (
	azureClientID           = "AZURE_CLIENT_ID"
	azureFederatedTokenFile = "AZURE_FEDERATED_TOKEN_FILE"
	azureTenantID           = "AZURE_TENANT_ID"
)

func NewConnectionStringbuilder(endpoint string) *kusto.ConnectionStringBuilder {
	var clientID, file, tenantID string

	useDefault := false
	ok := false

	if endpoint == "" {
		return nil
	}

	kustoConnectionStringBuilder := kusto.NewConnectionStringBuilder(endpoint)

	if clientID, ok = os.LookupEnv(azureClientID); !ok {
		useDefault = true
	}

	if file, ok = os.LookupEnv(azureFederatedTokenFile); !ok {
		useDefault = true
	}

	if tenantID, ok = os.LookupEnv(azureTenantID); !ok {
		useDefault = true
	}

	if useDefault {
		return kustoConnectionStringBuilder.WithDefaultAzureCredential()
	}

	return kustoConnectionStringBuilder.WithKubernetesWorkloadIdentity(clientID, file, tenantID)
}

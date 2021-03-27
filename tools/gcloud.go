package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"

	googleauth "golang.org/x/oauth2/google"
	cloudbilling "google.golang.org/api/cloudbilling/v1"

	gopipeline "github.com/mattn/go-pipeline"
)

type gcloudConfigOutput struct {
	Core map[string]string `json:"core"`
}

type appYaml struct {
	Runtime            string                 `yaml:"runtime"`
	InstanceClass      string                 `yaml:"instance_class"`
	AutomaticScaling   map[string]interface{} `yaml:"automatic_scaling"`
	VpcAccessConnector map[string]string      `yaml:"vpc_access_connector"`
	EnvVariables       map[string]string      `yaml:"env_variables"`
	Service            string                 `yaml:"service"`
}

var projectIDMemo *string = nil

const billingAccountNamePrefix = "billingAccounts/"

// GetProjectID detects GCP project ID from
// instance metadata (for GCE, GCF, GAE, ...) or gcloud CLI.
func GetProjectID(ctx context.Context) (string, error) {
	if projectIDMemo != nil {
		return *projectIDMemo, nil
	}
	creds, e := googleauth.FindDefaultCredentials(ctx)
	if e != nil {
		return "", e
	}

	if creds.ProjectID != "" {
		projectIDMemo = &creds.ProjectID
	} else {
		gcloudOutJSON, e := gopipeline.Output(
			[]string{"gcloud", "-q", "config", "list", "core/project", "--format=json"},
		)
		if e != nil {
			return "", fmt.Errorf("failed to invoke gcloud command: %v", e.Error())
		}
		var out gcloudConfigOutput
		if e = json.Unmarshal(gcloudOutJSON, &out); e != nil {
			return "", fmt.Errorf("failed to parse gcloud command output")
		}
		projectID := out.Core["project"]
		projectIDMemo = &projectID
	}

	log.Printf("using projectID: %v", *projectIDMemo)

	return *projectIDMemo, nil
}

// GetProjectBillingAccount returns name of billing account for the project.
func GetProjectBillingAccount(ctx context.Context) (string, error) {
	projectID, e := GetProjectID(ctx)
	if e != nil {
		return "", e
	}

	service, e := cloudbilling.NewService(ctx)
	if e != nil {
		return "", e
	}

	path := fmt.Sprintf("projects/%s", projectID)
	billingInfo, e := service.Projects.GetBillingInfo(path).Do()
	if e != nil {
		return "", e
	}
	accountName := billingInfo.BillingAccountName
	if strings.Index(accountName, billingAccountNamePrefix) != 0 {
		return "", fmt.Errorf("billing account name is unexpected format: %s", accountName)
	}

	result := accountName[len(billingAccountNamePrefix):]
	log.Printf("using billing account: %v", result)
	return result, nil
}

// GetGKECredentials calls `gcloud containers clusters get-credentials`
func GetGKECredentials(out io.Writer, clusterName string, clusterZone string) error {
	output, e := gopipeline.Output(
		[]string{"gcloud", "container", "clusters", "get-credentials", clusterName, "--zone", clusterZone},
	)
	if e != nil {
		return e
	}
	if _, e = out.Write(output); e != nil {
		return e
	}
	return nil
}

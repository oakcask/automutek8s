//+build mage

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"

	"cloud.google.com/go/storage"
	"github.com/magefile/mage/mg"
	"github.com/oakcask/automutek8s/tools"
	goyaml "gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	kustomize "sigs.k8s.io/kustomize/api/types"
)

var config tools.Config

func init() {
	f, e := os.Open("config.yaml")
	if e != nil {
		log.Fatal(e)
	}
	if yaml.NewYAMLOrJSONDecoder(f, 4096).Decode(&config) != e {
		log.Fatal(e)
	}
}

type Secrets mg.Namespace
type GKE mg.Namespace
type GAE mg.Namespace

func loadSecretManifests() ([]corev1.Secret, error) {
	glob := path.Join("kubernetes", "base", "secrets", "*.yaml")
	paths, e := filepath.Glob(glob)
	if e != nil {
		return nil, e
	}

	secrets := make([]corev1.Secret, 0)

	for _, path := range paths {
		f, e := os.Open(path)
		if e != nil {
			return nil, e
		}

		var manifest corev1.Secret
		if e = yaml.NewYAMLOrJSONDecoder(f, 4096).Decode(&manifest); e != nil {
			return nil, e
		}

		secrets = append(secrets, manifest)
	}
	return secrets, nil
}

// Show list of secrets in manifests
func (Secrets) List(ctx context.Context) error {
	secrets, e := loadSecretManifests()
	if e != nil {
		return e
	}

	for _, secret := range secrets {
		for _, handle := range tools.NewSecretHandles(secret) {
			hasSecret, e := handle.Exists(ctx)
			var value string
			if hasSecret {
				value = "<filtered>"
			} else if e != nil {
				log.Printf(e.Error())
				value = "ERROR"
			} else {
				value = "(none)"
			}
			fmt.Printf("%v = %v\n", handle.String(), value)
		}
	}

	return nil
}

// Store secret to cloud.
//
// Parameter valueOrFile will accepts raw value, absolute path or single hyphen (`-`).
// When an absolute path is given, the task will read secret from a file pointed by the path.
// When single hypthen is given, the task will read secret from stdin.
// Please note that the secret typed on tty WILL NOT be masked in this time
// so if you want to keep it hidden, please use pipe (like `cat secret.txt |`) or
// absolute path.
func (Secrets) Set(ctx context.Context, k8sSecretName string, key string, valueOrFile string) error {
	handle := tools.SecretHandle{
		MetadataName: k8sSecretName,
		Key:          key,
	}

	payload, e := tools.ReadFromStringOrPath(valueOrFile)
	if e != nil {
		return e
	}

	return handle.Set(ctx, payload)
}

// Get the secret from cloud.
func (Secrets) Unvail(ctx context.Context, k8sSecretName string, key string) error {
	handle := tools.SecretHandle{
		MetadataName: k8sSecretName,
		Key:          key,
	}

	stat, e := os.Stdout.Stat()
	if e != nil {
		return e
	}

	if (stat.Mode() & os.ModeCharDevice) != 0 {
		return fmt.Errorf("secret unvailing cancelled: stdout is char device (maybe tty?)")
	}

	payload, e := handle.Unvail(ctx)
	if e != nil {
		return e
	}

	if _, e = os.Stdout.Write(payload); e != nil {
		return e
	}
	if e := os.Stdout.Close(); e != nil {
		return e
	}

	return nil
}

// Generates terraform configuration
func Terraform(ctx context.Context) error {
	projectID, e := tools.GetProjectID(ctx)
	if e != nil {
		return e
	}
	gcsClient, e := storage.NewClient(ctx)
	if e != nil {
		return e
	}
	defer gcsClient.Close()

	backendBucket, e := tools.CreateOrGetTFBackendBucket(ctx, gcsClient)
	if e != nil {
		return e
	}

	tf, e := os.OpenFile("auto.tf.json", os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0600)
	if e != nil {
		return e
	}
	defer tf.Close()

	w := bufio.NewWriter(tf)
	if e != nil {
		return e
	}

	tfvars := tools.TFDocument{
		Terraform: tools.TFTerraform{
			Backend: tools.TFTerraformBackend{
				GCS: tools.TFObject{
					"bucket": backendBucket,
				},
			},
		},
		Variable: map[string]tools.TFVariable{
			"gcloud_project": {
				Default: projectID,
			},
			"region": {
				Default: config.Region,
			},
			"zone": {
				Default: config.Zone,
			},
			"cluster_name": {
				Default: config.Cluster.Name,
			},
			"cluster_location": {
				Default: config.Cluster.Location,
			},
			"primary_vpc_ip_cidr_range": {
				Default: config.Network.PrimaryCidrRange,
			},
			"primary_vpc_pod_ip_cidr_range": {
				Default: config.Network.PodCidrRange,
			},
			"primary_vpc_service_ip_cidr_range": {
				Default: config.Network.ServiceCidrRange,
			},
			"primary_cluster_master_cidr_block": {
				Default: config.Network.MasterCidrRange,
			},
			"ingress_ip_resource_id": {
				Default: config.Network.IngressIPResourceID,
			},
		},
	}
	b, e := json.MarshalIndent(tfvars, "", "  ")
	if e != nil {
		return e
	}
	if _, e = w.Write(b); e != nil {
		return e
	}

	e = w.Flush()
	if e != nil {
		return e
	}
	return nil
}

// Generate kustomization.yaml from template
func Kustomization(ctx context.Context) error {
	yamlfile, e := os.Open("kustomization.yaml.template")
	if e != nil {
		return e
	}
	templatedYamlFile, e := tools.ApplyTextTemplate(yamlfile, config)
	if e != nil {
		return e
	}

	var kustomization kustomize.Kustomization
	if e = yaml.NewYAMLOrJSONDecoder(templatedYamlFile, 4096).Decode(&kustomization); e != nil {
		return e
	}

	secrets, e := loadSecretManifests()
	if e != nil {
		return e
	}

	for _, secret := range secrets {
		var literalSources []string

		for _, handle := range tools.NewSecretHandles(secret) {
			payload, e := handle.Unvail(ctx)
			if e != nil {
				return e
			}

			literalSources = append(literalSources, fmt.Sprintf("%s=%s", handle.Key, string(payload)))
		}

		secretArgs := kustomize.SecretArgs{
			GeneratorArgs: kustomize.GeneratorArgs{
				Name:     secret.Name,
				Behavior: "create",
				KvPairSources: kustomize.KvPairSources{
					LiteralSources: literalSources,
				},
			},
		}
		kustomization.SecretGenerator = append(kustomization.SecretGenerator, secretArgs)
		kustomization.GeneratorOptions = &kustomize.GeneratorOptions{
			DisableNameSuffixHash: false,
		}
	}

	out, e := os.OpenFile("kustomization.yaml", os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0600)
	if e != nil {
		return e
	}
	defer out.Close()

	return goyaml.NewEncoder(out).Encode(kustomization)
}

// Setup credentials to kubectl
func (GKE) GetCredentials(ctx context.Context) error {
	return tools.GetGKECredentials(os.Stdout, config.Cluster.Name, config.Cluster.Location)
}

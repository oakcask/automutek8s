package tools

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"

	"encoding/hex"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
	corev1 "k8s.io/api/core/v1"
)

// SecretHandle hols information that points to
// the secret data which managed by cloud secret manager.
type SecretHandle struct {
	MetadataName string
	Key          string
}

// NewSecretHandles builds SecretHandles from k8s Secret.
func NewSecretHandles(secret corev1.Secret) []SecretHandle {
	handles := make([]SecretHandle, 0)

	for key := range secret.Data {
		handle := SecretHandle{
			MetadataName: secret.Name,
			Key:          key,
		}
		handles = append(handles, handle)
	}

	return handles
}

func (handle SecretHandle) buildCloudSecretLabels() map[string]string {
	return map[string]string{
		"automutek8s":       "v1",
		"k8s-metadata-name": handle.MetadataName,
		"key":               handle.Key,
	}
}

func (handle SecretHandle) buildCloudSecretParentPath(ctx context.Context) (string, error) {
	projectID, e := GetProjectID(ctx)
	if e != nil {
		return "", e
	}
	path := fmt.Sprintf("projects/%s", projectID)
	return path, nil
}

func (handle SecretHandle) buildCloudSecretName(ctx context.Context) (string, error) {
	if !IsValidK8sMetadataName(handle.MetadataName) {
		return "", fmt.Errorf("invalid name for Kubernates Secret: %s", handle.MetadataName)
	}
	if len(handle.Key) <= 0 {
		return "", fmt.Errorf("key name cannot be empty")
	}

	name := []byte(handle.String())
	sum := sha256.Sum256(name)
	sumSlize := sum[:]
	hexName := hex.EncodeToString(sumSlize)

	return fmt.Sprintf("automutek8s_%s", hexName), nil
}

func (handle SecretHandle) buildCloudSecretPath(ctx context.Context) (string, error) {
	parentPath, e := handle.buildCloudSecretParentPath(ctx)
	if e != nil {
		return "", e
	}

	name, e := handle.buildCloudSecretName(ctx)
	if e != nil {
		return "", e
	}

	return fmt.Sprintf("%s/secrets/%s", parentPath, name), nil
}

func (handle SecretHandle) buildCloudSecretVersionPath(ctx context.Context, version string) (string, error) {
	path, e := handle.buildCloudSecretPath(ctx)
	if e != nil {
		return "", e
	}

	return fmt.Sprintf("%s/versions/%s", path, version), nil
}

func (handle SecretHandle) String() string {
	return fmt.Sprintf("%v %v", handle.MetadataName, handle.Key)
}

func (handle SecretHandle) getSecret(ctx context.Context, client *secretmanager.Client) (*secretmanagerpb.Secret, error) {
	path, e := handle.buildCloudSecretPath(ctx)
	if e != nil {
		return nil, e
	}
	req := &secretmanagerpb.GetSecretRequest{
		Name: path,
	}

	return client.GetSecret(ctx, req)
}

func (handle SecretHandle) createSecret(ctx context.Context, client *secretmanager.Client) (*secretmanagerpb.Secret, error) {
	parent, e := handle.buildCloudSecretParentPath(ctx)
	if e != nil {
		return nil, e
	}
	name, e := handle.buildCloudSecretName(ctx)
	if e != nil {
		return nil, e
	}

	log.Printf("creating secret: %v", parent)
	req := &secretmanagerpb.CreateSecretRequest{
		Parent:   parent,
		SecretId: name,
		Secret: &secretmanagerpb.Secret{
			Replication: &secretmanagerpb.Replication{
				Replication: &secretmanagerpb.Replication_Automatic_{
					Automatic: &secretmanagerpb.Replication_Automatic{},
				},
			},
			Labels: map[string]string{
				"automutek8s": "",
			},
		},
	}

	return client.CreateSecret(ctx, req)
}

// Exists tests precense of the secret pointed by the SecretHandle.
func (handle SecretHandle) Exists(ctx context.Context) (bool, error) {
	client, e := secretmanager.NewClient(ctx)
	if e != nil {
		return false, e
	}
	defer client.Close()

	_, e = handle.getSecret(ctx, client)
	if e != nil {
		if IsGrpcNotFound(e) {
			return false, nil
		}

		return false, e
	}

	return true, nil
}

// Set stores payload to cloud for the secret pointed by the SecretHandle.
func (handle SecretHandle) Set(ctx context.Context, payload []byte) error {
	client, e := secretmanager.NewClient(ctx)
	if e != nil {
		return e
	}
	defer client.Close()

	secret, e := handle.getSecret(ctx, client)
	if e != nil && !IsGrpcNotFound(e) {
		return e
	}
	if e != nil {
		log.Printf("no secrets matching %v: %v", handle.String(), e.Error())
		secret, e = handle.createSecret(ctx, client)
		if e != nil {
			return e
		}
	}

	log.Printf("adding secret version...")
	addReq := &secretmanagerpb.AddSecretVersionRequest{
		Parent: secret.GetName(),
		Payload: &secretmanagerpb.SecretPayload{
			Data: payload,
		},
	}

	_, e = client.AddSecretVersion(ctx, addReq)

	return e
}

// Unvail retrieves secret version from cloud, and
// returns its payload.
func (handle SecretHandle) Unvail(ctx context.Context) ([]byte, error) {
	client, e := secretmanager.NewClient(ctx)
	if e != nil {
		return nil, e
	}
	defer client.Close()

	path, e := handle.buildCloudSecretVersionPath(ctx, "latest")
	if e != nil {
		return nil, e
	}
	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: path,
	}
	secretVersion, e := client.AccessSecretVersion(ctx, req)
	if e != nil {
		return nil, e
	}

	return secretVersion.Payload.Data, nil
}

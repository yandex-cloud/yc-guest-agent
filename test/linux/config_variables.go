package linux

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"github.com/gruntwork-io/terratest/modules/aws"
	"github.com/gruntwork-io/terratest/modules/terraform"
	"marketplace-yaga/test/utils"
	"os"
	"strings"
	"testing"
)

func configureTerraformOptions(t *testing.T, exampleFolder string) (*terraform.Options, *aws.Ec2Keypair) {
	// A unique ID we can use to namespace resources, so we don't clash with anything already in the folder or
	// tests running in parallel
	uniqueID := utils.UniqueId()

	// Give this Instance and other resources in the Terraform code a name with a unique ID, so it doesn't clash
	// with anything else in the folder.
	instanceName := fmt.Sprintf("yaga-kms-%s", uniqueID)
	instanceSaName := fmt.Sprintf("yaga-sa-%s", uniqueID)

	// Create the Key Pair that we can use for SSH access
	randR := rand.Reader
	publicKey, privateKey, _ := ed25519.GenerateKey(randR)
	keyPair, _ := utils.PairFromED25519(publicKey, privateKey)

	user := "ubuntu"

	// Construct the terraform options with default retryable errors to handle the most common retryable errors in
	// terraform testing.
	terraformOptions := terraform.WithDefaultRetryableErrors(t, &terraform.Options{
		// The path to where our Terraform code is located
		TerraformDir: exampleFolder,

		// Variables to pass to our Terraform code using -var options
		Vars: map[string]interface{}{
			"instance_name":    instanceName,
			"instance_sa_name": instanceSaName,
			"public_ssh_key":   strings.Trim(keyPair.PublicKey, "\n "),
			"folder_id":        os.Getenv("YC_FOLDER_ID"),
			"cloud_id":         os.Getenv("YC_CLOUD_ID"),
			"sa_key":           os.Getenv("YC_SA_KEY"),
			"user":             user,
		},
	})

	return terraformOptions, &aws.Ec2Keypair{
		KeyPair: keyPair,
		Name:    "yaga",
		Region:  "ru-central1",
	}
}

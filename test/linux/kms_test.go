package linux

import (
	"crypto/ed25519"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	cryptorand "crypto/rand"
	"github.com/gruntwork-io/terratest/modules/aws"
	"github.com/gruntwork-io/terratest/modules/retry"
	"github.com/gruntwork-io/terratest/modules/ssh"
	"github.com/gruntwork-io/terratest/modules/terraform"
	test_structure "github.com/gruntwork-io/terratest/modules/test-structure"
)

func TestKms(t *testing.T) {
	t.Parallel()

	exampleFolder := test_structure.CopyTerraformFolderToTemp(t, "./", "kms")

	// At the end of the test, run `terraform destroy` to clean up any resources that were created
	defer test_structure.RunTestStage(t, "teardown", func() {
		terraformOptions := test_structure.LoadTerraformOptions(t, exampleFolder)
		terraform.Destroy(t, terraformOptions)
	})

	// Deploy the example
	test_structure.RunTestStage(t, "setup", func() {
		terraformOptions, keyPair := configureTerraformOptions(t, exampleFolder)

		// Save the options and key pair so later test stages can use them
		test_structure.SaveTerraformOptions(t, exampleFolder, terraformOptions)
		test_structure.SaveEc2KeyPair(t, exampleFolder, keyPair)

		// This will run `terraform init` and `terraform apply` and fail the test if there are any errors
		terraform.InitAndApply(t, terraformOptions)
	})

	// Make sure we can SSH to the public Instance directly from the public Internet and the private Instance by using
	// the public Instance as a jump host
	test_structure.RunTestStage(t, "validate", func() {
		terraformOptions := test_structure.LoadTerraformOptions(t, exampleFolder)
		savedKeyPair := test_structure.LoadEc2KeyPair(t, exampleFolder)

		testSSHToPublicHost(t, terraformOptions, savedKeyPair.KeyPair)
	})

}

func configureTerraformOptions(t *testing.T, exampleFolder string) (*terraform.Options, *aws.Ec2Keypair) {
	// A unique ID we can use to namespace resources, so we don't clash with anything already in the folder or
	// tests running in parallel
	uniqueID := UniqueId()

	// Give this Instance and other resources in the Terraform code a name with a unique ID, so it doesn't clash
	// with anything else in the folder.
	instanceName := fmt.Sprintf("yaga-kms-%s", uniqueID)
	instanceSaName := fmt.Sprintf("yaga-sa-%s", uniqueID)

	// Create the Key Pair that we can use for SSH access
	randR := cryptorand.Reader
	publicKey, privateKey, _ := ed25519.GenerateKey(randR)
	keyPair, _ := PairFromED25519(publicKey, privateKey)

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

func testSSHToPublicHost(t *testing.T, terraformOptions *terraform.Options, keyPair *ssh.KeyPair) {
	// Run `terraform output` to get the value of an output variable
	publicInstanceIP := terraform.Output(t, terraformOptions, "public_instance_ip")

	// We're going to try to SSH to the instance IP, using the Key Pair we created earlier, and the user
	publicHost := ssh.Host{
		Hostname:    publicInstanceIP,
		SshKeyPair:  keyPair,
		SshUserName: terraformOptions.Vars["user"].(string),
	}

	// It can take a minute or so for the Instance to boot up, so retry a few times
	maxRetries := 30
	timeBetweenRetries := 5 * time.Second
	description := fmt.Sprintf("SSH to YAGA host %s", publicInstanceIP)

	remoteTempFilePath := "/tmp/yaga"
	expectedText := "much secret! very encrypted!"
	runCommand := fmt.Sprintf("sudo %s start </dev/null 2>&1 >/var/log/yaga &", remoteTempFilePath)

	localFileName := "../../linux/yandex-guest-agent/yandex-guest-agent"
	localFile, err := os.ReadFile(localFileName)
	if err != nil {
		t.Fatalf("Error: reading local file: %s", err.Error())
	}
	// Wait until SSH is available
	ssh.CheckSshConnectionWithRetry(t, publicHost, maxRetries, timeBetweenRetries)

	// Run commands
	retry.DoWithRetry(t, description, maxRetries, timeBetweenRetries, func() (string, error) {
		ssh.ScpFileTo(t, publicHost, 0744, remoteTempFilePath, string(localFile))

		ssh.CheckSshCommand(t, publicHost, runCommand)
		return "", nil
	})
	// Verify contents of the created file
	retry.DoWithRetry(t, description, 5, timeBetweenRetries, func() (string, error) {
		actualText, err := ssh.FetchContentsOfFileE(t, publicHost, true, "/opt/yaga/secret")

		if err != nil {
			return "", err
		}

		if strings.TrimSpace(actualText) != expectedText {
			return "", fmt.Errorf("Expected YAGA file to return '%s' but got '%s'", expectedText, actualText)
		}

		return "", nil
	})

}

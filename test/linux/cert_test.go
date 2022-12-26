package linux

import (
	"fmt"
	"marketplace-yaga/test/goreleaser"
	"marketplace-yaga/test/utils"
	"os"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/retry"
	"github.com/gruntwork-io/terratest/modules/ssh"
	"github.com/gruntwork-io/terratest/modules/terraform"
	test_structure "github.com/gruntwork-io/terratest/modules/test-structure"
)

func TestCert(t *testing.T) {
	t.Parallel()

	exampleFolder := test_structure.CopyTerraformFolderToTemp(t, "./", "cert")

	// At the end of the test, run `terraform destroy` to clean up any resources that were created
	defer test_structure.RunTestStage(t, "teardown", func() {
		terraformOptions := test_structure.LoadTerraformOptions(t, exampleFolder)
		terraform.Destroy(t, terraformOptions)
	})

	// Deploy the example
	test_structure.RunTestStage(t, "setup", func() {
		terraformOptions, keyPair := configureTerraformOptions(t, exampleFolder, "cert")
		certPair := utils.GenerateCertificates()
		terraformOptions.Vars["certificate"] = certPair.Certificate
		terraformOptions.Vars["private_key"] = certPair.PrivateKey

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

		testValidateCertViaSSH(t, terraformOptions, savedKeyPair.KeyPair)
	})

}

func testValidateCertViaSSH(t *testing.T, terraformOptions *terraform.Options, keyPair *ssh.KeyPair) {
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

	packageArtifactInfo := goreleaser.FindLinuxPackage(goreleaser.ParseArtefacts("../../dist/artifacts.json"))

	remoteTempFilePath := "/tmp/" + packageArtifactInfo.Name
	expectedText := terraformOptions.Vars["certificate"].(string) + terraformOptions.Vars["private_key"].(string)
	installCommand := fmt.Sprintf("sudo dpkg --install %s", remoteTempFilePath)

	localFile, err := os.ReadFile("../../" + packageArtifactInfo.Path)
	if err != nil {
		t.Fatalf("Error: reading local file: %s", err.Error())
	}
	// Wait until SSH is available
	ssh.CheckSshConnectionWithRetry(t, publicHost, maxRetries, timeBetweenRetries)

	// Run commands
	retry.DoWithRetry(t, description, maxRetries, timeBetweenRetries, func() (string, error) {
		ssh.ScpFileTo(t, publicHost, 0744, remoteTempFilePath, string(localFile))

		ssh.CheckSshCommand(t, publicHost, installCommand)
		return "", nil
	})
	// Verify contents of the created file
	retry.DoWithRetry(t, description, 5, timeBetweenRetries, func() (string, error) {
		actualText, err := ssh.FetchContentsOfFileE(t, publicHost, true, "/etc/nginx/ssl/yaga.pem")

		if err != nil {
			return "", err
		}

		if actualText != expectedText {
			return "", fmt.Errorf("Expected YAGA file to return '%s' but got '%s'", expectedText, actualText)
		}

		return "", nil
	})

}

package linux

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"
	ycsdk "github.com/yandex-cloud/go-sdk"
	"github.com/yandex-cloud/go-sdk/iamkey"
	"log"
	"marketplace-yaga/pkg/messages"
	"marketplace-yaga/test/goreleaser"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/retry"
	"github.com/gruntwork-io/terratest/modules/ssh"
	"github.com/gruntwork-io/terratest/modules/terraform"
	test_structure "github.com/gruntwork-io/terratest/modules/test-structure"
)

type request struct {
	Modulus  string
	Exponent string
	Username string
	Expires  int64
	Schema   string
}

type response struct {
	Modulus           string
	Exponent          string
	Username          string
	EncryptedPassword string
	Success           bool
	Error             string
}

func TestUsers(t *testing.T) {
	t.Parallel()

	exampleFolder := test_structure.CopyTerraformFolderToTemp(t, "./", "users")

	// At the end of the test, run `terraform destroy` to clean up any resources that were created
	defer test_structure.RunTestStage(t, "teardown", func() {
		terraformOptions := test_structure.LoadTerraformOptions(t, exampleFolder)
		terraform.Destroy(t, terraformOptions)
	})

	// Deploy the example
	test_structure.RunTestStage(t, "setup", func() {
		terraformOptions, keyPair := configureTerraformOptions(t, exampleFolder, "users")

		// Save the options and key pair so later test stages can use them
		test_structure.SaveTerraformOptions(t, exampleFolder, terraformOptions)
		test_structure.SaveEc2KeyPair(t, exampleFolder, keyPair)

		// This will run `terraform init` and `terraform apply` and fail the test if there are any errors
		terraform.InitAndApply(t, terraformOptions)
	})

	// Make sure we can SSH to the public Instance directly from the public Internet and the private Instance by using
	// the public Instance as a jump host
	test_structure.RunTestStage(t, "precheck", func() {
		terraformOptions := test_structure.LoadTerraformOptions(t, exampleFolder)
		savedKeyPair := test_structure.LoadEc2KeyPair(t, exampleFolder)

		testPrecheckUsers(t, terraformOptions, savedKeyPair.KeyPair)
	})

	// Make sure we can update SSH keys and still connect to VM
	test_structure.RunTestStage(t, "validate", func() {
		terraformOptions := test_structure.LoadTerraformOptions(t, exampleFolder)
		savedKeyPair := test_structure.LoadEc2KeyPair(t, exampleFolder)

		testValidateUsers(t, terraformOptions, savedKeyPair.KeyPair)
	})

}

func testPrecheckUsers(t *testing.T, terraformOptions *terraform.Options, keyPair *ssh.KeyPair) {
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
}

func testValidateUsers(t *testing.T, terraformOptions *terraform.Options, keyPair *ssh.KeyPair) {
	// Run `terraform output` to get the value of an output variable
	publicInstanceIP := terraform.Output(t, terraformOptions, "public_instance_ip")
	publicInstanceId := terraform.Output(t, terraformOptions, "public_instance_id")

	// We're going to try to SSH to the instance IP, using the Key Pair we created earlier, and the user
	publicHost := ssh.Host{
		Hostname:    publicInstanceIP,
		SshKeyPair:  keyPair,
		SshUserName: terraformOptions.Vars["user"].(string),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var saKey iamkey.Key

	err := json.Unmarshal([]byte(terraformOptions.Vars["sa_key"].(string)), &saKey)
	if err != nil {
		return
	}

	creds, _ := ycsdk.ServiceAccountKey(&saKey)
	sdk, err := ycsdk.Build(ctx, ycsdk.Config{
		Credentials: creds,
	})
	if err != nil {
		log.Fatal(err)
	}

	rsaKey, _ := rsa.GenerateKey(rand.Reader, 4096)

	exponent := base64.StdEncoding.EncodeToString(big.NewInt(int64(rsaKey.PublicKey.E)).Bytes())
	modulus := base64.StdEncoding.EncodeToString(rsaKey.PublicKey.N.Bytes())

	req := request{
		Modulus:  modulus,
		Exponent: exponent,
		Username: "yc",
		Expires:  time.Now().Unix(),
		Schema:   "v1",
	}
	e := messages.NewEnvelope().WithType("UserChangeRequest")
	m, err := e.Marshal(req)

	op, err := sdk.WrapOperation(sdk.Compute().Instance().UpdateMetadata(ctx, &compute.UpdateInstanceMetadataRequest{
		InstanceId: publicInstanceId,
		Upsert: map[string]string{
			"linux-users": string(m),
		},
	}))
	if err != nil {
		return
	}
	err = op.Wait(ctx)
	if err != nil {
		return
	}
	// It can take a minute or so for the Instance to boot up, so retry a few times
	maxRetries := 10
	timeBetweenRetries := 5 * time.Second

	var pass string
	for i := 0; i < maxRetries; i++ {
		time.Sleep(timeBetweenRetries)
		res, err := sdk.Compute().Instance().GetSerialPortOutput(context.Background(), &compute.GetInstanceSerialPortOutputRequest{
			InstanceId: publicInstanceId,
			Port:       4,
		})
		if err != nil {
			return
		}
		output := parseOutput(res.Contents)
		if output != nil {
			if !output.Success {
				t.Fatalf("failed to reset password: %s", output.Error)
			}
			pass, _ = decryptPassword(rsaKey, output.EncryptedPassword)
			if pass != "" {
				break
			}
		}
		t.Logf("retrying to get password %d retry", i)
	}
	println(pass)

	grepPassCmd := fmt.Sprintf(`sudo cat /etc/shadow | grep %s`, req.Username)

	retry.DoWithRetry(t, "Trying to validate password",
		maxRetries, timeBetweenRetries,
		func() (string, error) {
			res := ssh.CheckSshCommand(t, publicHost, grepPassCmd)
			hashedPass := strings.Split(res, ":")[1]
			splitted := strings.Split(hashedPass, "$")

			checkPassCmd := fmt.Sprintf(`echo "%s" | openssl passwd -%s -salt %s -stdin`, pass, splitted[1], splitted[2])
			checkRes := ssh.CheckSshCommand(t, publicHost, checkPassCmd)

			if !strings.Contains(res, strings.Trim(checkRes, "\n ")) {
				t.Logf(`"%s"\n"%s"`, res, checkRes)
				t.Fatalf("password check failed")
			}
			return "", nil
		})

}

func decryptPassword(rsaKey *rsa.PrivateKey, encPwd string) (pwd string, err error) {
	var bsPwd []byte
	if bsPwd, err = base64.StdEncoding.DecodeString(encPwd); err != nil {
		return
	}

	var decPwd []byte
	if decPwd, err = rsa.DecryptOAEP(sha256.New(), rand.Reader, rsaKey, bsPwd, nil); err != nil {
		return
	}
	pwd = string(decPwd)

	return
}

type envelope struct {
	Timestamp int    `json:"Timestamp"`
	Type      string `json:"Type"`
	ID        string `json:"ID"`
	Payload   json.RawMessage
}

func parseOutput(data string) *response {
	lines := strings.Split(data, "\n")
	var en envelope
	for _, l := range lines {
		err := json.Unmarshal([]byte(l), &en)
		if err != nil {
			return nil
		}
		if en.Type == "UserChangeResponse" {
			var res response
			err := json.Unmarshal(en.Payload, &res)
			if err != nil {
				return nil
			}
			return &res
		}
	}
	return nil
}

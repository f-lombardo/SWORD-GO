package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"

	"sword-go/internal/cloud/digitalocean"
	"sword-go/internal/cloud/hetzner"
)

const (
	doAPISecretPath      = "/run/secrets/do_api_key"
	hetznerAPISecretPath = "/run/secrets/hetzner_api_key"
)

func Run(ctx context.Context, stdout io.Writer, stderr io.Writer, args []string) error {
	if len(args) < 2 {
		printMainUsage(stderr)
		return errors.New("missing command")
	}

	switch args[0] {
	case "digitalocean":
		if args[1] != "create" {
			printMainUsage(stderr)
			return fmt.Errorf("unknown subcommand: digitalocean %s", args[1])
		}

		return runDigitalOceanCreate(ctx, stdout, stderr, args[2:])
	case "hetzner":
		if args[1] != "create" {
			printMainUsage(stderr)
			return fmt.Errorf("unknown subcommand: hetzner %s", args[1])
		}

		return runHetznerCreate(ctx, stdout, stderr, args[2:])
	default:
		printMainUsage(stderr)
		return fmt.Errorf("unknown provider: %s", args[0])
	}
}

func runDigitalOceanCreate(ctx context.Context, stdout io.Writer, stderr io.Writer, args []string) error {
	flags := flag.NewFlagSet("digitalocean create", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	key := flags.String("key", "", "The API key")
	name := flags.String("name", "", "The droplet name")
	region := flags.String("region", "nyc1", "The region")
	serverType := flags.String("type", "s-1vcpu-1gb", "The droplet type")
	image := flags.String("image", "ubuntu-24-04-x64", "The image")
	publicKey := flags.String("public-key", "", "Name of an existing DigitalOcean SSH key, or a raw public key")

	if err := flags.Parse(args); err != nil {
		return err
	}

	apiKey := resolveAPIKey(*key, doAPISecretPath, "DIGITALOCEAN_TOKEN", stdout, "DigitalOcean API not found in /run/secrets/do-api.", "Failed to read DigitalOcean API key from /run/secrets/do-api.")
	if strings.TrimSpace(apiKey) == "" {
		return errors.New("an API key is required. Pass --key=<token>, provide it in /run/secrets/do-api, or set DIGITALOCEAN_TOKEN in your environment")
	}

	if strings.TrimSpace(*publicKey) == "" {
		return errors.New("--public-key is required. Provide an existing DigitalOcean key name or a raw public key string")
	}

	dropletName := *name
	if strings.TrimSpace(dropletName) == "" {
		dropletName = "droplet-" + randomLowerAlphaNumeric(8)
	}

	fmt.Fprintln(stdout, "Resolving SSH key…")

	creator := digitalocean.NewCreator(http.DefaultClient)
	result, err := creator.Create(ctx, digitalocean.CreateDropletData{
		APIKey:                   apiKey,
		Name:                     dropletName,
		ServerType:               *serverType,
		Region:                   *region,
		Image:                    *image,
		PublicKey:                *publicKey,
		PublicIPPollAttempts:     30,
		PublicIPPollIntervalSecs: 5,
	})
	if err != nil {
		return err
	}

	if result.SSHKeyStatus == "uploaded" && result.SSHKeyName != nil {
		fmt.Fprintf(stdout, "SSH key %s uploaded to DigitalOcean.\n", *result.SSHKeyName)
	}

	if result.SSHKeyStatus == "existing" && result.SSHKeyName != nil {
		fmt.Fprintf(stdout, "Using existing DigitalOcean SSH key %s.\n", *result.SSHKeyName)
	}

	fmt.Fprintln(stdout, "Droplet created successfully.")

	table := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(table, "ID\tName\tRegion\tSize\tStatus")
	_, _ = fmt.Fprintf(table, "%d\t%s\t%s\t%s\t%s\n", result.DropletID, result.Name, result.Region, result.Type, result.Status)
	_ = table.Flush()

	if result.PublicIP == nil {
		return errors.New("droplet is active but no public IP was assigned within the timeout")
	}

	fmt.Fprintf(stdout, "Public IP: %s\n", *result.PublicIP)
	return nil
}

func runHetznerCreate(ctx context.Context, stdout io.Writer, stderr io.Writer, args []string) error {
	flags := flag.NewFlagSet("hetzner create", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	key := flags.String("key", "", "The API key")
	name := flags.String("name", "", "The server name")
	location := flags.String("location", "nbg1", "The Hetzner location")
	serverType := flags.String("type", "cx23", "The Hetzner server type")
	image := flags.String("image", "ubuntu-24.04", "The image")
	publicKey := flags.String("public-key", "", "Name of an existing Hetzner SSH key, or a raw public key")

	if err := flags.Parse(args); err != nil {
		return err
	}

	apiKey := resolveAPIKey(*key, hetznerAPISecretPath, "HETZNER_TOKEN", stdout, "Hetzner API not found in /run/secrets/hetzner-api.", "Failed to read Hetzner API key from /run/secrets/hetzner-api.")
	if strings.TrimSpace(apiKey) == "" {
		return errors.New("an API key is required. Pass --key=<token>, provide it in /run/secrets/hetzner-api, or set HETZNER_TOKEN in your environment")
	}

	if strings.TrimSpace(*publicKey) == "" {
		return errors.New("--public-key is required. Provide an existing Hetzner SSH key name or a raw public key string")
	}

	serverName := *name
	if strings.TrimSpace(serverName) == "" {
		serverName = "server-" + randomLowerAlphaNumeric(8)
	}

	fmt.Fprintln(stdout, "Resolving SSH key…")

	creator := hetzner.NewCreator(http.DefaultClient)
	result, err := creator.Create(ctx, hetzner.CreateServerData{
		APIKey:                   apiKey,
		Name:                     serverName,
		ServerType:               *serverType,
		Location:                 *location,
		Image:                    *image,
		PublicKey:                *publicKey,
		PublicIPPollAttempts:     30,
		PublicIPPollIntervalSecs: 5,
	})
	if err != nil {
		return err
	}

	if result.SSHKeyStatus == "uploaded" && result.SSHKeyName != nil {
		fmt.Fprintf(stdout, "SSH key %s uploaded to Hetzner.\n", *result.SSHKeyName)
	}

	if result.SSHKeyStatus == "existing" && result.SSHKeyName != nil {
		fmt.Fprintf(stdout, "Using existing Hetzner SSH key %s.\n", *result.SSHKeyName)
	}

	fmt.Fprintln(stdout, "Server created successfully.")

	table := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(table, "ID\tName\tLocation\tType\tStatus")
	_, _ = fmt.Fprintf(table, "%d\t%s\t%s\t%s\t%s\n", result.ServerID, result.Name, result.Location, result.ServerType, result.Status)
	_ = table.Flush()

	if result.PublicIP == nil {
		return errors.New("server is active but no public IPv4 was assigned within the timeout")
	}

	fmt.Fprintf(stdout, "Public IP: %s\n", *result.PublicIP)
	return nil
}

func resolveAPIKey(commandLineAPIKey string, secretPath string, envName string, stdout io.Writer, notFoundMessage string, failedMessage string) string {
	if strings.TrimSpace(commandLineAPIKey) != "" {
		return strings.TrimSpace(commandLineAPIKey)
	}

	secretFileAPIKey := readAPIKeyFromSecret(secretPath, stdout, notFoundMessage, failedMessage)
	if secretFileAPIKey != "" {
		return secretFileAPIKey
	}

	return strings.TrimSpace(os.Getenv(envName))
}

func readAPIKeyFromSecret(secretPath string, stdout io.Writer, notFoundMessage string, failedMessage string) string {
	info, err := os.Stat(secretPath)
	if err != nil || info.IsDir() {
		fmt.Fprintln(stdout, notFoundMessage)
		return ""
	}

	content, err := os.ReadFile(secretPath)
	if err != nil {
		fmt.Fprintln(stdout, failedMessage)
		return ""
	}

	return strings.TrimSpace(string(content))
}

func printMainUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "Usage:")
	_, _ = fmt.Fprintln(w, "  sword-go digitalocean create [--key=TOKEN] [--name=NAME] [--region=nyc1] [--type=s-1vcpu-1gb] [--image=ubuntu-24-04-x64] --public-key=<name|key>")
	_, _ = fmt.Fprintln(w, "  sword-go hetzner create [--key=TOKEN] [--name=NAME] [--location=nbg1] [--type=cx23] [--image=ubuntu-24.04] --public-key=<name|key>")
}

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
	"sword-go/internal/utils"
	"text/tabwriter"

	"sword-go/internal/cloud/digitalocean"
	"sword-go/internal/cloud/hetzner"
)

const (
	doAPISecretPath      = "/run/secrets/do_api_key"
	hetznerAPISecretPath = "/run/secrets/hetzner_api_key"
)

func Run(ctx context.Context, stdout io.Writer, stderr io.Writer, args []string) error {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help") {
		printMainUsage(stdout)
		return nil
	}
	if len(args) < 2 {
		printMainUsage(stderr)
		return errors.New("missing command")
	}

	switch args[0] {
	case "digitalocean":
		switch args[1] {
		case "create":
			return runDigitalOceanCreate(ctx, stdout, stderr, args[2:])
		case "regions":
			return runDigitalOceanListRegions(ctx, stdout, stderr, args[2:])
		case "images":
			return runDigitalOceanListImages(ctx, stdout, stderr, args[2:])
		case "-h", "--help", "help":
			printMainUsage(stdout)
			return nil
		default:
			printMainUsage(stderr)
			return fmt.Errorf("unknown subcommand: digitalocean %s", args[1])
		}
	case "hetzner":
		switch args[1] {
		case "create":
			return runHetznerCreate(ctx, stdout, stderr, args[2:])
		case "regions":
			return runHetznerListRegions(ctx, stdout, stderr, args[2:])
		case "images":
			return runHetznerListImages(ctx, stdout, stderr, args[2:])
		case "-h", "--help", "help":
			printMainUsage(stdout)
			return nil
		default:
			printMainUsage(stderr)
			return fmt.Errorf("unknown subcommand: hetzner %s", args[1])
		}
	default:
		printMainUsage(stderr)
		return fmt.Errorf("unknown provider: %s", args[0])
	}
}

func runDigitalOceanListRegions(ctx context.Context, stdout io.Writer, stderr io.Writer, args []string) error {
	flags := flag.NewFlagSet("digitalocean regions", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage:")
		fmt.Fprintln(stderr, "  sword-go digitalocean regions [--key=TOKEN]")
	}

	key := flags.String("key", "", "The API key")

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	apiKey, err := resolveAPIKey(*key, doAPISecretPath, "DIGITALOCEAN_TOKEN")
	if err != nil {
		return err
	}

	provider := digitalocean.NewProvider(http.DefaultClient)
	regions, err := provider.ListAvailableRegions(ctx, apiKey)
	if err != nil {
		return err
	}

	sizes, err := provider.ListSizes(ctx, apiKey)
	if err != nil {
		return err
	}

	for _, region := range regions {
		fmt.Fprintf(stdout, "%-8s %s\n", region.Slug, region.Name)
		availableSizes := region.ListAvailable(sizes)
		for _, size := range availableSizes {
			fmt.Fprintf(stdout, "\t%s\n", size.String())
		}
	}
	return nil
}

func runDigitalOceanListImages(ctx context.Context, stdout io.Writer, stderr io.Writer, args []string) error {
	flags := flag.NewFlagSet("digitalocean images", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage:")
		fmt.Fprintln(stderr, "  sword-go digitalocean images [--key=TOKEN]")
	}

	key := flags.String("key", "", "The API key")

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	apiKey, err := resolveAPIKey(*key, doAPISecretPath, "DIGITALOCEAN_TOKEN")
	if err != nil {
		return err
	}

	provider := digitalocean.NewProvider(http.DefaultClient)
	images, err := provider.ListAvailableImages(ctx, apiKey)
	if err != nil {
		return err
	}

	table := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(table, "SLUG\tDISTRIBUTION\tNAME")
	for _, img := range images {
		slug := img.Slug
		if slug == "" {
			slug = fmt.Sprintf("%d", img.ID)
		}
		_, _ = fmt.Fprintf(table, "%s\t%s\t%s\n", slug, img.Distribution, img.Name)
	}
	return table.Flush()
}

func runDigitalOceanCreate(ctx context.Context, stdout io.Writer, stderr io.Writer, args []string) error {
	flags := flag.NewFlagSet("digitalocean create", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage:")
		fmt.Fprintln(stderr, "  sword-go digitalocean create [--key=TOKEN] [--name=NAME] [--region=nyc1] [--type=s-1vcpu-1gb] [--image=ubuntu-24-04-x64] --public-key=<name|key>")
	}

	key := flags.String("key", "", "The API key")
	name := flags.String("name", "", "The droplet name")
	region := flags.String("region", "nyc1", "The region")
	serverType := flags.String("type", "s-1vcpu-1gb", "The droplet type")
	image := flags.String("image", "ubuntu-24-04-x64", "The image")
	publicKey := flags.String("public-key", "", "Name of an existing DigitalOcean SSH key, or a raw public key")

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	apiKey, err := resolveAPIKey(*key, doAPISecretPath, "DIGITALOCEAN_TOKEN")
	if err != nil {
		return err
	}

	if strings.TrimSpace(*publicKey) == "" {
		return errors.New("--public-key is required. Provide an existing DigitalOcean key name or a raw public key string")
	}

	dropletName := *name
	if strings.TrimSpace(dropletName) == "" {
		dropletName = "droplet-" + utils.RandomLowerAlphaNumeric(8)
	}

	fmt.Fprintln(stdout, "Resolving SSH key…")

	provider := digitalocean.NewProvider(http.DefaultClient)
	result, err := provider.Create(ctx, digitalocean.CreateDropletData{
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
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage:")
		fmt.Fprintln(stderr, "  sword-go hetzner create [--key=TOKEN] [--name=NAME] [--location=nbg1] [--type=cx23] [--image=ubuntu-24.04] --public-key=<name|key>")
	}

	key := flags.String("key", "", "The API key")
	name := flags.String("name", "", "The server name")
	location := flags.String("location", "nbg1", "The Hetzner location")
	serverType := flags.String("type", "cx23", "The Hetzner server type")
	image := flags.String("image", "ubuntu-24.04", "The image")
	publicKey := flags.String("public-key", "", "Name of an existing Hetzner SSH key, or a raw public key")

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	apiKey, err := resolveAPIKey(*key, hetznerAPISecretPath, "HETZNER_TOKEN")
	if err != nil {
		return err
	}

	if strings.TrimSpace(*publicKey) == "" {
		return errors.New("--public-key is required. Provide an existing Hetzner SSH key name or a raw public key string")
	}

	serverName := *name
	if strings.TrimSpace(serverName) == "" {
		serverName = "server-" + utils.RandomLowerAlphaNumeric(8)
	}

	fmt.Fprintln(stdout, "Resolving SSH key…")

	provider := hetzner.NewProvider(http.DefaultClient)
	result, err := provider.Create(ctx, hetzner.CreateServerData{
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

func runHetznerListRegions(ctx context.Context, stdout io.Writer, stderr io.Writer, args []string) error {
	flags := flag.NewFlagSet("hetzner regions", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage:")
		fmt.Fprintln(stderr, "  sword-go hetzner regions [--key=TOKEN]")
	}

	key := flags.String("key", "", "The API key")

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	apiKey, err := resolveAPIKey(*key, hetznerAPISecretPath, "HETZNER_TOKEN")
	if err != nil {
		return err
	}
	provider := hetzner.NewProvider(http.DefaultClient)
	regions, err := provider.ListAvailableLocations(ctx, apiKey)
	if err != nil {
		return err
	}

	serverTypes, err := provider.ListServerTypes(ctx, apiKey)
	if err != nil {
		return err
	}

	for _, region := range regions {
		fmt.Fprintf(stdout, "%-4s %s %s\n", region.Name, region.Description, region.Country)
		availableServerTypes := region.ListAvailable(serverTypes)
		for _, size := range availableServerTypes {
			fmt.Fprintf(stdout, "\t%s\n", size.String())
		}
	}
	return nil
}

func runHetznerListImages(ctx context.Context, stdout io.Writer, stderr io.Writer, args []string) error {
	flags := flag.NewFlagSet("hetzner images", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage:")
		fmt.Fprintln(stderr, "  sword-go hetzner images [--key=TOKEN]")
	}

	key := flags.String("key", "", "The API key")

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	apiKey, err := resolveAPIKey(*key, hetznerAPISecretPath, "HETZNER_TOKEN")
	if err != nil {
		return err
	}

	provider := hetzner.NewProvider(http.DefaultClient)
	images, err := provider.ListAvailableImages(ctx, apiKey)
	if err != nil {
		return err
	}

	table := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(table, "NAME\tARCH\tDESCRIPTION")
	for _, img := range images {
		name := ""
		if img.Name != nil {
			name = *img.Name
		}
		if name == "" {
			name = fmt.Sprintf("%d", img.ID)
		}
		arch := img.Architecture
		if arch == "" {
			arch = "-"
		}
		_, _ = fmt.Fprintf(table, "%s\t%s\t%s\n", name, arch, img.Description)
	}
	return table.Flush()
}

func resolveAPIKey(commandLineAPIKey string, secretPath string, envName string) (string, error) {
	if strings.TrimSpace(commandLineAPIKey) != "" {
		return strings.TrimSpace(commandLineAPIKey), nil
	}

	secretFileAPIKey := readAPIKeyFromSecret(secretPath)
	if secretFileAPIKey != "" {
		return secretFileAPIKey, nil
	}

	envApiKey := strings.TrimSpace(os.Getenv(envName))
	if envApiKey != "" {
		return envApiKey, nil
	}

	return "", fmt.Errorf("an API key is required. Pass --key=<token>, provide it in %s, or set %s in your environment", secretPath, envName)
}

func readAPIKeyFromSecret(secretPath string) string {
	info, err := os.Stat(secretPath)
	if err != nil || info.IsDir() {
		return ""
	}

	content, err := os.ReadFile(secretPath)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(content))
}

func printMainUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "Usage:")
	_, _ = fmt.Fprintln(w, "  sword-go digitalocean create [--key=TOKEN] [--name=NAME] [--region=nyc1] [--type=s-1vcpu-1gb] [--image=ubuntu-24-04-x64] --public-key=<name|key>")
	_, _ = fmt.Fprintln(w, "  sword-go digitalocean regions [--key=TOKEN]")
	_, _ = fmt.Fprintln(w, "  sword-go digitalocean images [--key=TOKEN]")
	_, _ = fmt.Fprintln(w, "  sword-go hetzner create [--key=TOKEN] [--name=NAME] [--location=nbg1] [--type=cx23] [--image=ubuntu-24.04] --public-key=<name|key>")
	_, _ = fmt.Fprintln(w, "  sword-go hetzner regions [--key=TOKEN]")
	_, _ = fmt.Fprintln(w, "  sword-go hetzner images [--key=TOKEN]")
	_, _ = fmt.Fprintln(w, "  sword-go web")
}

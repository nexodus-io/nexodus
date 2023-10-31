package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/olekukonko/tablewriter"
	"log"
	"net/url"
	"os"
	"reflect"
	"runtime"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/api/public"
	"github.com/nexodus-io/nexodus/internal/client"
	"github.com/urfave/cli/v2"
)

const (
	encodeJsonRaw    = "json-raw"
	encodeJsonPretty = "json"
	encodeNoHeader   = "no-header"
	encodeColumn     = "column"
)

// This variable is set using ldflags at build time. See Makefile for details.
var Version = "dev"

// Optionally set at build time using ldflags
var DefaultServiceURL = "https://try.nexodus.io"

var additionalPlatformCommands []*cli.Command = nil

func main() {
	// Override usage to capitalize "Show"
	cli.HelpFlag.(*cli.BoolFlag).Usage = "Show help"
	app := &cli.App{
		Name:                 "nexctl",
		Usage:                "controls the Nexodus control and data planes",
		EnableBashCompletion: true,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "debug",
				Value:   false,
				Usage:   "Enable debug logging",
				EnvVars: []string{"NEXCTL_DEBUG"},
			},
			&cli.StringFlag{
				Name:   "host",
				Value:  DefaultServiceURL,
				Usage:  "Api server URL",
				Hidden: true,
			},
			&cli.StringFlag{
				Name:  "service-url",
				Value: DefaultServiceURL,
				Usage: "Api server URL",
			},
			&cli.StringFlag{
				Name:  "username",
				Usage: "Username",
			},
			&cli.StringFlag{
				Name:  "password",
				Usage: "Password",
			},
			&cli.StringFlag{
				Name:     "output",
				Value:    encodeColumn,
				Required: false,
				Usage:    "Output format: json, json-raw, no-header, column (default columns)",
			},
			&cli.BoolFlag{
				Name:     "insecure-skip-tls-verify",
				Value:    false,
				Usage:    "If true, server certificates will not be checked for validity. This will make your HTTPS connections insecure",
				Required: false,
			},
		},
		Commands: []*cli.Command{
			{
				Name:  "version",
				Usage: "Get the version of nexctl",
				Action: func(cCtx *cli.Context) error {
					fmt.Printf("version: %s\n", Version)
					return nil
				},
			},
			{
				Name:  "registration-token",
				Usage: "Commands relating to registration tokens",
				Subcommands: []*cli.Command{
					{
						Name:  "list",
						Usage: "List registration tokens",
						Action: func(cCtx *cli.Context) error {
							return listRegistrationTokens(cCtx, mustCreateAPIClient(cCtx))
						},
					},
					{
						Name:  "create",
						Usage: "Create a registration token",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "organization-id",
								Required: false,
							},
							&cli.StringFlag{
								Name:     "description",
								Required: false,
							},
							&cli.BoolFlag{
								Name:     "single-use",
								Required: false,
							},
							&cli.DurationFlag{
								Name:     "expiration",
								Required: false,
							},
						},
						Action: func(cCtx *cli.Context) error {
							return createRegistrationToken(cCtx, mustCreateAPIClient(cCtx), public.ModelsAddRegistrationToken{
								OrganizationId: cCtx.String("organization-id"),
								Description:    cCtx.String("description"),
								Expiration:     toExpiration(cCtx.Duration("expiration")),
								SingleUse:      cCtx.Bool("single-use"),
							})
						},
					},
					{
						Name:  "delete",
						Usage: "Delete a registration token",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "id",
								Required: true,
							},
						},
						Action: func(cCtx *cli.Context) error {
							encodeOut := cCtx.String("output")
							id := cCtx.String("id")
							return deleteRegistrationToken(cCtx, mustCreateAPIClient(cCtx), encodeOut, id)
						},
					},
				},
			},
			{
				Name:  "organization",
				Usage: "Commands relating to organizations",
				Subcommands: []*cli.Command{
					{
						Name:  "list",
						Usage: "List organizations",
						Action: func(cCtx *cli.Context) error {
							return listOrganizations(cCtx, mustCreateAPIClient(cCtx))
						},
					},
					{
						Name:  "create",
						Usage: "Create a organizations",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "name",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "description",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "cidr",
								Required: false,
							},
							&cli.StringFlag{
								Name:     "cidr-v6",
								Required: false,
							},
						},
						Action: func(cCtx *cli.Context) error {
							organizationName := cCtx.String("name")
							organizationDescrip := cCtx.String("description")
							organizationCIDR := cCtx.String("cidr")
							organizationCIDRv6 := cCtx.String("cidr-v6")
							return createOrganization(cCtx, mustCreateAPIClient(cCtx), organizationName, organizationDescrip, organizationCIDR, organizationCIDRv6)
						},
					},
					{
						Name:  "delete",
						Usage: "Delete a organization",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "organization-id",
								Required: true,
							},
						},
						Action: func(cCtx *cli.Context) error {
							encodeOut := cCtx.String("output")
							organizationID := cCtx.String("organization-id")
							return deleteOrganization(cCtx, mustCreateAPIClient(cCtx), encodeOut, organizationID)
						},
					},
					{
						Name:        "metadata",
						Usage:       "Commands relating to device metadata across the organization",
						Subcommands: organizationMetadataSubcommands,
					},
				},
			},
			{
				Name:  "device",
				Usage: "Commands relating to devices",
				Subcommands: []*cli.Command{
					{
						Name:  "list",
						Usage: "List all devices",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "organization-id",
								Value:    "",
								Required: false,
							},
							&cli.BoolFlag{
								Name:    "full",
								Aliases: []string{"f"},
								Usage:   "display the full set of device details",
								Value:   false,
							},
						},
						Action: func(cCtx *cli.Context) error {
							orgID := cCtx.String("organization-id")
							if orgID != "" {
								id, err := uuid.Parse(orgID)
								if err != nil {
									log.Fatal(err)
								}
								return listOrgDevices(cCtx, mustCreateAPIClient(cCtx), id)
							}
							return listAllDevices(cCtx, mustCreateAPIClient(cCtx))
						},
					},
					{
						Name:  "delete",
						Usage: "Delete a device",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "device-id",
								Required: true,
							},
						},
						Action: func(cCtx *cli.Context) error {
							encodeOut := cCtx.String("output")
							devID := cCtx.String("device-id")
							return deleteDevice(mustCreateAPIClient(cCtx), encodeOut, devID)
						},
					},
					{
						Name:        "metadata",
						Usage:       "Commands relating to device metadata",
						Subcommands: deviceMetadataSubcommands,
					},
				},
			},
			{
				Name:  "user",
				Usage: "Commands relating to users",
				Subcommands: []*cli.Command{
					{
						Name:  "list",
						Usage: "List all users",
						Action: func(cCtx *cli.Context) error {
							return listUsers(cCtx, mustCreateAPIClient(cCtx))
						},
					},
					{
						Name:  "get-current",
						Usage: "Get current user",
						Action: func(cCtx *cli.Context) error {
							return getCurrent(cCtx, mustCreateAPIClient(cCtx))
						},
					},
					{
						Name:  "delete",
						Usage: "Delete a user",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "user-id",
								Required: true,
								Hidden:   true,
							},
						},
						Action: func(cCtx *cli.Context) error {
							encodeOut := cCtx.String("output")
							userID := cCtx.String("user-id")
							return deleteUser(cCtx, mustCreateAPIClient(cCtx), encodeOut, userID)
						},
					},
					{
						Name:  "remove-user",
						Usage: "Remove a user from an organization",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "user-id",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "organization-id",
								Required: true,
							},
						},
						Action: func(cCtx *cli.Context) error {
							encodeOut := cCtx.String("output")
							userID := cCtx.String("user-id")
							orgID := cCtx.String("organization-id")
							return deleteUserFromOrg(cCtx, mustCreateAPIClient(cCtx), encodeOut, userID, orgID)
						},
					},
				},
			},
			{
				Name:  "security-group",
				Usage: "commands relating to security groups",
				Subcommands: []*cli.Command{
					{
						Name:  "list",
						Usage: "List all security groups",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "organization-id",
								Required: true,
							},
						},
						Action: func(cCtx *cli.Context) error {
							encodeOut := cCtx.String("output")
							orgID := cCtx.String("organization-id")
							return listSecurityGroups(cCtx, mustCreateAPIClient(cCtx), encodeOut, orgID)
						},
					},
					{
						Name:  "delete",
						Usage: "Delete a security group",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "security-group-id",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "organization-id",
								Required: true,
							},
						},
						Action: func(cCtx *cli.Context) error {
							encodeOut := cCtx.String("output")
							sgID := cCtx.String("security-group-id")
							orgID := cCtx.String("organization-id")
							return deleteSecurityGroup(cCtx, mustCreateAPIClient(cCtx), encodeOut, sgID, orgID)
						},
					},
					{
						Name:  "create",
						Usage: "create a security group",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "name",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "organization-id",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "description",
								Required: false,
							},
							&cli.StringFlag{
								Name:     "inbound-rules",
								Required: false,
							},
							&cli.StringFlag{
								Name:     "outbound-rules",
								Required: false,
							},
						},
						Action: func(cCtx *cli.Context) error {
							name := cCtx.String("name")
							description := cCtx.String("description")
							orgID := cCtx.String("organization-id")
							inboundRulesStr := cCtx.String("inbound-rules")
							outboundRulesStr := cCtx.String("outbound-rules")

							var inboundRules, outboundRules []public.ModelsSecurityRule
							var err error

							if inboundRulesStr != "" {
								inboundRules, err = jsonStringToSecurityRules(inboundRulesStr)
								if err != nil {
									return fmt.Errorf("failed to convert inbound rules string to security rules: %w", err)
								}
							}

							if outboundRulesStr != "" {
								outboundRules, err = jsonStringToSecurityRules(outboundRulesStr)
								if err != nil {
									return fmt.Errorf("failed to convert outbound rules string to security rules: %w", err)
								}
							}

							return createSecurityGroup(cCtx, mustCreateAPIClient(cCtx), name, description, orgID, inboundRules, outboundRules)
						},
					},
					{
						Name:  "update",
						Usage: "update a security group",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "name",
								Required: false,
							},
							&cli.StringFlag{
								Name:     "security-group-id",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "organization-id",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "description",
								Required: false,
							},
							&cli.StringFlag{
								Name:     "inbound-rules",
								Required: false,
							},
							&cli.StringFlag{
								Name:     "outbound-rules",
								Required: false,
							},
						},
						Action: func(cCtx *cli.Context) error {
							name := cCtx.String("name")
							sgID := cCtx.String("security-group-id")
							orgID := cCtx.String("organization-id")
							description := cCtx.String("description")
							inboundRulesStr := cCtx.String("inbound-rules")
							outboundRulesStr := cCtx.String("outbound-rules")

							var inboundRules, outboundRules []public.ModelsSecurityRule
							var err error

							if inboundRulesStr != "" {
								inboundRules, err = jsonStringToSecurityRules(inboundRulesStr)
								if err != nil {
									return fmt.Errorf("failed to convert inbound rules string to security rules: %w", err)
								}
							}

							if outboundRulesStr != "" {
								outboundRules, err = jsonStringToSecurityRules(outboundRulesStr)
								if err != nil {
									return fmt.Errorf("failed to convert outbound rules string to security rules: %w", err)
								}
							}

							return updateSecurityGroup(cCtx, mustCreateAPIClient(cCtx), sgID, orgID, name, description, inboundRules, outboundRules)
						},
					},
				},
			},
			{
				Name:  "invitation",
				Usage: "commands relating to invitations",
				Subcommands: []*cli.Command{
					{
						Name:  "create",
						Usage: "create an invitation",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "user-id",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "organization-id",
								Required: false,
							},
						},
						Action: func(cCtx *cli.Context) error {
							encodeOut := cCtx.String("output")
							userID := cCtx.String("user-id")
							orgID := cCtx.String("organization-id")
							return createInvitation(mustCreateAPIClient(cCtx), encodeOut, userID, orgID)
						},
					},
					{
						Name:  "delete",
						Usage: "delete an invitation",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "inv-id",
								Required: true,
							},
						},
						Action: func(cCtx *cli.Context) error {
							userID := cCtx.String("inv-id")
							return deleteInvitation(mustCreateAPIClient(cCtx), userID)
						},
					},
					{
						Name:  "accept",
						Usage: "accept an invitation",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "inv-id",
								Required: true,
							},
						},
						Action: func(cCtx *cli.Context) error {
							userID := cCtx.String("inv-id")
							return acceptInvitation(mustCreateAPIClient(cCtx), userID)
						},
					},
				},
			},
		},
	}

	app.Commands = append(app.Commands, additionalPlatformCommands...)
	sort.Slice(app.Commands, func(i, j int) bool {
		return app.Commands[i].Name < app.Commands[j].Name
	})

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func toExpiration(duration time.Duration) string {
	if duration == 0 {
		return ""
	}
	return time.Now().Add(duration).String()
}

func mustCreateAPIClient(cCtx *cli.Context) *client.APIClient {

	urlValue := DefaultServiceURL
	flagUsed := "--service-url"
	addApiPrefix := true
	if cCtx.IsSet("host") {
		if cCtx.IsSet("service-url") {
			log.Fatalf("please remove the --host flag, the --service-url flag has replaced it")
		}
		log.Println("DEPRECATION WARNING: configuring the service url via the --host flag not be supported in a future release.  Please use the --service-url flag instead.")
		urlValue = cCtx.String("host")
		flagUsed = "--host"
		addApiPrefix = false
	} else if cCtx.IsSet("service-url") {
		urlValue = cCtx.String("service-url")
	}

	apiURL, err := url.Parse(urlValue)
	if err != nil {
		log.Fatalf("invalid '%s=%s' flag provided. error: %v", flagUsed, urlValue, err)
	}
	if apiURL.Scheme != "https" {
		log.Fatalf("invalid '%s=%s' flag provided. error: 'https://' URL scheme is required", flagUsed, urlValue)
	}

	if addApiPrefix {
		apiURL.Host = "api." + apiURL.Host
		apiURL.Path = ""
	}

	c, err := client.NewAPIClient(cCtx.Context,
		apiURL.String(), nil,
		createClientOptions(cCtx)...,
	)
	if err != nil {
		log.Fatal(err)
	}
	return c
}

func createClientOptions(cCtx *cli.Context) []client.Option {
	options := []client.Option{
		client.WithPasswordGrant(
			cCtx.String("username"),
			cCtx.String("password"),
		),
		client.WithUserAgent(fmt.Sprintf("nexctl/%s (%s; %s)", Version, runtime.GOOS, runtime.GOARCH)),
	}
	if cCtx.Bool("insecure-skip-tls-verify") { // #nosec G402
		options = append(options, client.WithTLSConfig(&tls.Config{
			InsecureSkipVerify: true,
		}))
	}
	return options
}

func newTabWriter() *tabwriter.Writer {
	return tabwriter.NewWriter(os.Stdout, 10, 1, 5, ' ', 0)
}

func FormatOutput(format string, result interface{}) error {
	switch format {
	case encodeJsonPretty:
		bytes, err := json.MarshalIndent(result, "", "    ")
		if err != nil {
			log.Fatalf("failed to encode the ctl output: %v", err)
		}
		fmt.Println(string(bytes))

	case encodeJsonRaw:
		bytes, err := json.Marshal(result)
		if err != nil {
			log.Fatalf("failed to encode the ctl output: %v", err)
		}
		fmt.Println(string(bytes))

	default:
		return fmt.Errorf("unknown format option: %s", format)
	}

	return nil
}

func showOutput(cCtx *cli.Context, fields []TableField, result any) {
	output := cCtx.String("output")
	switch output {
	case encodeJsonPretty:
		bytes, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			log.Fatalf("failed to encode the ctl output: %v", err)
		}
		fmt.Println(string(bytes))

	case encodeJsonRaw:
		bytes, err := json.Marshal(result)
		if err != nil {
			log.Fatalf("failed to encode the ctl output: %v", err)
		}
		fmt.Println(string(bytes))

	case encodeColumn, encodeNoHeader:

		table := tablewriter.NewWriter(os.Stdout)
		table.SetBorders(tablewriter.Border{
			Left:   true,
			Right:  true,
			Top:    false,
			Bottom: false,
		})
		table.SetAutoWrapText(false)

		if output != encodeNoHeader {
			var headers []string
			for _, field := range fields {
				headers = append(headers, field.Header)
			}
			table.SetHeader(headers)
		}

		itemsValue := reflect.ValueOf(result)
		if itemsValue.IsNil() {
			table.Render()
			return
		}
		// if the itemsValue is not a slice, lets turn it into one.
		if itemsValue.Type().Kind() != reflect.Slice {
			itemsValue = reflect.MakeSlice(reflect.SliceOf(itemsValue.Type()), 0, 1)
			itemsValue = reflect.Append(itemsValue, reflect.ValueOf(result))
		}
		for i := 0; i < itemsValue.Len(); i++ {
			itemValue := itemsValue.Index(i)
			var line []string
			for _, field := range fields {
				if field.Formatter != nil {
					line = append(line, field.Formatter(itemValue.Interface()))
				} else if field.Field != "" {
					// Deref the items points.
					for itemValue.Type().Kind() == reflect.Pointer {
						itemValue = itemValue.Elem()
					}
					fieldValue := itemValue.FieldByName(field.Field)
					line = append(line, fieldFormatter(fieldValue))
				} else {
					panic("TableField.Formatter or TableField.Field must be set")
				}
			}
			table.Append(line)
		}
		table.Render()
		return
	default:
		log.Fatalf("unknown --output option: %s", output)
	}
}

func fieldFormatter(itemValue reflect.Value) string {
	switch itemValue.Type().Kind() {
	case reflect.Invalid:
		return ""
	case reflect.Pointer:
		// deref and try again...
		return fieldFormatter(itemValue.Elem())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fmt.Sprintf("%d", itemValue.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return fmt.Sprintf("%d", itemValue.Uint())
	case reflect.Float32, reflect.Float64:
		return fmt.Sprintf("%f", itemValue.Float())
	case reflect.Bool:
		return fmt.Sprintf("%v", itemValue.Bool())

	case reflect.String:
		return itemValue.String()
	default:
		item := itemValue.Interface()
		if item, ok := item.([]byte); ok {
			return string(item)
		}
		bytes, err := json.MarshalIndent(item, "", " ")
		if err != nil {
			panic(err)
		}
		return string(bytes)
	}
}

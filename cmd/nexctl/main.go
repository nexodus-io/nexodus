package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ghodss/yaml"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/api/public"
	"github.com/olekukonko/tablewriter"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"runtime"
	"sort"
	"time"

	"github.com/nexodus-io/nexodus/internal/client"
	"github.com/urfave/cli/v2"
)

const (
	encodeJsonRaw    = "json-raw"
	encodeJsonPretty = "json"
	encodeYaml       = "yaml"
	encodeNoHeader   = "no-header"
	encodeColumn     = "column"
)

// Version is set using ldflags at build time. See Makefile for details.
var Version = "dev"

// DefaultServiceURL is optionally set at build time using ldflags
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
				Usage:    "Output format: json, json-raw, yaml, no-header, column (default columns)",
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
			createRegKeyCommand(),
			createOrganizationCommand(),
			createVpcCommand(),
			createDeviceCommand(),
			createUserSubCommand(),
			createSecurityGroupCommand(),
			createInvitationCommand(),
		},
	}

	app.Commands = append(app.Commands, additionalPlatformCommands...)
	sort.Slice(app.Commands, func(i, j int) bool {
		return app.Commands[i].Name < app.Commands[j].Name
	})

	if err := app.Run(os.Args); err != nil {
		Fatal(err)
	}
}

func Fatal(a ...any) {
	fmt.Fprintln(os.Stderr, a...)
	os.Exit(1)
}
func Fatalf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(1)
}

func createClient(cCtx *cli.Context) *client.APIClient {

	urlValue := DefaultServiceURL
	flagUsed := "--service-url"
	addApiPrefix := true
	if cCtx.IsSet("host") {
		if cCtx.IsSet("service-url") {
			Fatal("please remove the --host flag, the --service-url flag has replaced it")
		}
		fmt.Fprintln(os.Stderr, "DEPRECATION WARNING: configuring the service url via the --host flag not be supported in a future release.  Please use the --service-url flag instead.")
		urlValue = cCtx.String("host")
		flagUsed = "--host"
		addApiPrefix = false
	} else if cCtx.IsSet("service-url") {
		urlValue = cCtx.String("service-url")
	}

	apiURL, err := url.Parse(urlValue)
	if err != nil {
		Fatalf("invalid '%s=%s' flag provided. error: %v", flagUsed, urlValue, err)
	}
	if apiURL.Scheme != "https" {
		Fatalf("invalid '%s=%s' flag provided. error: 'https://' URL scheme is required", flagUsed, urlValue)
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
		Fatal(err)
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

type TableField struct {
	Header    string
	Field     string
	Formatter func(item interface{}) string
}

func show(cCtx *cli.Context, fields []TableField, result any) {
	output := cCtx.String("output")
	switch output {
	case encodeJsonPretty:
		bytes, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			Fatalf("failed to encode the ctl output: %v", err)
		}
		fmt.Println(string(bytes))

	case encodeJsonRaw:
		bytes, err := json.Marshal(result)
		if err != nil {
			Fatalf("failed to encode the ctl output: %v", err)
		}
		fmt.Println(string(bytes))

	case encodeYaml:
		bytes, err := yaml.Marshal(result)
		if err != nil {
			Fatalf("failed to encode the ctl output: %v", err)
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
					if !fieldValue.IsValid() {
						panic(fmt.Sprintf("field %s not found", field.Field))
					}
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
		Fatalf("unknown --output option: %s", output)
	}
}

func showSuccessfully(ctx *cli.Context, action string) {
	encodeOut := ctx.String("output")
	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		fmt.Printf("\nsuccessfully %s\n", action)
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

func apiResponse[T any](resp T, httpResp *http.Response, err error) T {
	if err != nil {
		var openAPIError *public.GenericOpenAPIError
		switch {
		case errors.As(err, &openAPIError):
			model := openAPIError.Model()
			switch err := model.(type) {
			case public.ModelsBaseError:
				Fatalf("error: %s, status: %d", err.Error, httpResp.StatusCode)
			case public.ModelsConflictsError:
				Fatalf("error: %s: conflicting id: %s, status: %d", err.Error, err.Id, httpResp.StatusCode)
			case public.ModelsNotAllowedError:
				message := fmt.Sprintf("error: %s", err.Error)
				if err.Reason != "" {
					message += fmt.Sprintf(", reason: %s", err.Reason)
				}
				message += fmt.Sprintf(", status: %d", httpResp.StatusCode)
				Fatalf(message)
			case public.ModelsValidationError:
				message := fmt.Sprintf("error: %s", err.Error)
				if err.Field != "" {
					message += fmt.Sprintf(", field: %s", err.Field)
				}
				message += fmt.Sprintf(", status: %d", httpResp.StatusCode)
				Fatalf(message)
			case public.ModelsInternalServerError:
				Fatalf("error: %s: trace id: %s, status: %d", err.Error, err.TraceId, httpResp.StatusCode)
			default:
				Fatalf("error: %s, status: %d", string(openAPIError.Body()), httpResp.StatusCode)
			}
		default:
			Fatalf("error: %+v, status: %d", err, httpResp.StatusCode)
		}
	}
	return resp
}

func getUUID(ctx *cli.Context, name string) (string, error) {
	value := ctx.String(name)
	if value == "" {
		return "", nil
	}
	_, err := uuid.Parse(value)
	if err != nil {
		return "", fmt.Errorf("invalid value for --%s flag: %w", name, err)
	}
	return value, nil
}

func getExpiration(ctx *cli.Context, name string) string {
	value := ctx.Duration(name)
	if value == 0 {
		return ""
	}
	return time.Now().Add(value).String()
}

func getJsonMap(ctx *cli.Context, name string) (map[string]interface{}, error) {
	value := ctx.String(name)
	if value == "" {
		return nil, nil
	}
	valueMap := map[string]interface{}{}
	err := json.Unmarshal([]byte(value), &valueMap)
	if err != nil {
		return nil, fmt.Errorf("invalid value for --%s flag: not a json object: %w", name, err)

	}
	return valueMap, nil
}

func getDefaultOrgId(ctx context.Context, c *client.APIClient) string {
	user := apiResponse(c.UsersApi.GetUser(ctx, "me").Execute())
	return user.Id
}

func getDefaultVpcId(ctx context.Context, c *client.APIClient) string {
	user := apiResponse(c.UsersApi.GetUser(ctx, "me").Execute())
	return user.Id
}

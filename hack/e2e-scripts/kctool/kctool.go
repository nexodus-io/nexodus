package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Nerzal/gocloak/v13"
	"github.com/google/uuid"
	"github.com/urfave/cli/v2"
)

type AppConfig struct {
	CreateUser bool
	DeleteUser bool
	KcHost     string
	KcUsername string
	KcPassword string
	User       string
	Password   string
}

const (
	userRealm   = "nexodus"
	masterRealm = "master"
)

func main() {
	app := &cli.App{
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "create-user",
				Aliases: []string{"c"},
				Usage:   "create a new user",
			},
			&cli.BoolFlag{
				Name:    "delete-user",
				Aliases: []string{"d"},
				Usage:   "delete an existing user",
			},
			&cli.StringFlag{
				Name:     "kc-username",
				Aliases:  []string{"ku"},
				Usage:    "Keycloak admin username",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "kc-password",
				Aliases:  []string{"kp"},
				Usage:    "Keycloak admin password",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "user",
				Aliases:  []string{"u"},
				Usage:    "Username of User ID for the user",
				Required: true,
			},
			&cli.StringFlag{
				Name:    "password",
				Aliases: []string{"p"},
				Usage:   "Password for the new user",
			},
		},
		Action: func(c *cli.Context) error {
			// Check if there is an argument for the Keycloak host.
			if c.Args().Len() != 1 {
				return cli.Exit("Please provide the Keycloak host URL as the last argument.", 1)
			}

			kcHost := c.Args().First()

			config := &AppConfig{
				CreateUser: c.Bool("create-user"),
				DeleteUser: c.Bool("delete-user"),
				KcHost:     kcHost,
				KcUsername: c.String("kc-username"),
				KcPassword: c.String("kc-password"),
				User:       c.String("user"),
				Password:   c.String("password"),
			}

			if config.CreateUser && config.DeleteUser {
				return cli.Exit("please provide either --create-user or --delete-user, not both.", 1)
			}

			if config.CreateUser {
				if config.Password == "" {
					return cli.Exit("the flag --password is required for creating a user.", 1)
				}
				id, _ := uuid.NewUUID()
				config.User = config.User + id.String()
				config.createUser()
			} else if config.DeleteUser {
				config.deleteUser()
			} else {
				_ = cli.ShowAppHelp(c)
			}

			return nil
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func (config *AppConfig) createUser() {
	ctx, done := context.WithTimeout(context.Background(), time.Second*10)
	defer done()

	userID, err := config.createNewUser(ctx)

	if err != nil {
		log.Fatalf("Failed to create a new user: %v", err)
	} else {
		fmt.Println(userID)
	}
}

func (config *AppConfig) createNewUser(ctx context.Context) (string, error) {
	keycloak := gocloak.NewClient(config.KcHost)

	//nolint:gosec // This line is ignored because it's a test function
	keycloak.RestyClient().SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})

	token, err := keycloak.LoginAdmin(ctx, config.KcUsername, config.KcPassword, masterRealm)
	if err != nil {
		return "", fmt.Errorf("admin login to keycloak failed: %w", err)
	}

	userID, err := keycloak.CreateUser(ctx, token.AccessToken, userRealm, gocloak.User{
		FirstName: gocloak.StringP("Test"),
		LastName:  gocloak.StringP(config.User),
		Email:     gocloak.StringP(config.User + "@redhat.com"),
		Enabled:   gocloak.BoolP(true),
		Username:  gocloak.StringP(config.User),
	})
	if err != nil {
		return "", fmt.Errorf("user creation failed: %w", err)
	}

	err = keycloak.SetPassword(ctx, token.AccessToken, userID, userRealm, config.Password, false)
	if err != nil {
		return "", fmt.Errorf("user password setting failed: %w", err)
	}

	return config.User, nil
}

func (config *AppConfig) deleteUser() {
	ctx, done := context.WithTimeout(context.Background(), time.Second*10)
	defer done()

	err := config.deleteKeycloakUser(ctx)
	if err != nil {
		log.Fatalf("Failed to delete user: %v", err)
	} else {
		fmt.Printf("Successfully deleted user ID %s\n", config.User)
	}
}

func (config *AppConfig) deleteKeycloakUser(ctx context.Context) error {
	keycloak := gocloak.NewClient(config.KcHost)

	//nolint:gosec // This line is ignored because it's a test function
	keycloak.RestyClient().SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})

	token, err := keycloak.LoginAdmin(ctx, config.KcUsername, config.KcPassword, masterRealm)
	if err != nil {
		return fmt.Errorf("admin login to keycloak failed: %w", err)
	}

	err = keycloak.DeleteUser(ctx, token.AccessToken, userRealm, config.User)
	if err != nil {
		return fmt.Errorf("failed to delete user %s on realm: %s err: %w", config.User, userRealm, err)
	}

	return nil
}

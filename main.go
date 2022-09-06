package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/joho/godotenv"
	"github.com/urfave/cli/v2"
)

type SSMRunCommand interface {
	SendCommand(ctx context.Context, input *ssm.SendCommandInput, optFns ...func(*ssm.Options)) (*ssm.SendCommandOutput, error)
}

// Pull the Git repository for Locust scripts
func GitPullCommand(repo string, ctx context.Context, client SSMRunCommand, optFns ...func(*ssm.Options)) (*ssm.SendCommandOutput, error) {
	branch := os.Getenv("GIT_BRANCH")
	userName := os.Getenv("GIT_USERNAME")
	token := os.Getenv("GIT_TOKEN")
	fmt.Println("Pulling git repo: " + repo)
	return client.SendCommand(ctx, &ssm.SendCommandInput{
		DocumentName: aws.String("AWS-RunShellScript"),
		Comment:      aws.String("Git Pull"),
		Parameters: map[string][]string{
			"commands": {
				"cd /opt/locust",
				"git clone -b " + branch + " https://" + userName + ":" + token + "@" + repo,
			},
		},
		Targets: []types.Target{
			{
				Key:    aws.String("tag:Locust"),
				Values: []string{"true"},
			},
		},
	})
}

// Run Locust master service on Locust Master instance.
func RunLocustMasterCommand(ctx context.Context, client SSMRunCommand, optFns ...func(*ssm.Options)) (*ssm.SendCommandOutput, error) {
	scriptPath := os.Getenv("SCRIPT_PATH")
	webPort := os.Getenv("LOCUST_WEB_PORT")
	return client.SendCommand(ctx, &ssm.SendCommandInput{
		DocumentName: aws.String("AWS-RunShellScript"),
		Comment:      aws.String("Run Locust Master"),
		Parameters: map[string][]string{
			"commands": {
				"cd /opt/locust",
				"cd $(ls -d */|head -n 1)",
				"git fetch && git pull",
				"screen -dm bash -c 'locust -f " + scriptPath + " --web-port=" + webPort + " --master'",
			},
		},
		Targets: []types.Target{
			{
				Key:    aws.String("tag:LocustState"),
				Values: []string{"master"},
			},
		},
	}, optFns...)
}

// Run Locust worker services on Locust worker instances
func RunLocustWorkerCommand(ctx context.Context, client SSMRunCommand, optFns ...func(*ssm.Options)) (*ssm.SendCommandOutput, error) {
	scriptPath := os.Getenv("SCRIPT_PATH")
	fmt.Printf("Running workers...\n")
	// Describe master instance private IP address
	config, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(os.Getenv("REGION")))
	if err != nil {
		panic("AWS configuration error, " + err.Error())
	}

	ec2Client := ec2.NewFromConfig(config)
	//fmt.Printf("Describing master instance...\n")
	// input := &ec2.DescribeInstancesInput{
	// 	Filters: []types.Filter{
	// 		{
	// 			Name:   aws.String("tag:LocustState"),
	// 			Values: []string{"master"},
	// 		},
	// 	},
	// }
	// fmt.Printf("Describing master instance...\n")
	result, err := ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{})
	if err != nil {
		panic("Error describing instances, " + err.Error())
	}
	// fmt.Printf("%+v\n", result)
	for _, reservation := range result.Reservations {
		for _, instance := range reservation.Instances {
			fmt.Printf("%+v\n", instance)
			for _, tag := range instance.Tags {
				if *tag.Key == "LocustState" && *tag.Value == "master" {
					// fmt.Printf("%+v\n", instance)
					// fmt.Printf("%+v\n", instance.PrivateIpAddress)
					fmt.Printf("Worker run command sent successfully.\n")
					return client.SendCommand(ctx, &ssm.SendCommandInput{
						DocumentName: aws.String("AWS-RunShellScript"),
						Comment:      aws.String("Run Locust Workers"),
						Parameters: map[string][]string{
							"commands": {
								"cd /opt/locust",
								"cd $(ls -d */|head -n 1)",
								"git fetch && git pull",
								"screen -dm bash -c 'locust -f " + scriptPath + " --master-host=" + *instance.PrivateIpAddress + " --worker'",
							},
						},
						Targets: []types.Target{
							{
								Key:    aws.String("tag:LocustState"),
								Values: []string{"worker"},
							},
						},
					})
				}
			}
		}
	}
	// if len(result.Reservations) == 0 {
	// 	panic("No instances found")
	// }
	// if len(result.Reservations[0].Instances) == 0 {
	// 	panic("No instances found")
	// }
	// fmt.Printf("Described master instance...\n")
	// for _, reservation := range result.Reservations {
	// 	fmt.Print(reservation.Instances[0].PrivateIpAddress)
	// }
	// return client.SendCommand(ctx, &ssm.SendCommandInput{
	// 	DocumentName: aws.String("AWS-RunShellScript"),
	// 	Comment:      aws.String("Run Locust Worker"),
	// 	Parameters: map[string][]string{
	// 		"commands": {
	// 			"cd /opt/locust",
	// 			"cd $(ls -d */|head -n 1)",
	// 			"git fetch && git pull",
	// 			"screen -dm bash -c 'locust -f " + scriptPath + " --master-host=" + "test" + " --worker'",
	// 		},
	// 	},
	// 	Targets: []types.Target{
	// 		{
	// 			Key:    aws.String("tag:Name"),
	// 			Values: []string{"nginx"},
	// 		},
	// 	},
	// })
	return nil, err
}

// Repull git repository
func RePullGitRepoCommand(ctx context.Context, client SSMRunCommand, optFns ...func(*ssm.Options)) (*ssm.SendCommandOutput, error) {
	return client.SendCommand(ctx, &ssm.SendCommandInput{
		DocumentName: aws.String("AWS-RunShellScript"),
		Comment:      aws.String("Re-Pull Git Repo"),
		Parameters: map[string][]string{
			"commands": {
				"cd /opt/locust",
				"cd $(ls -d */|head -n 1)",
				"git fetch",
				"git pull",
			},
		},
		Targets: []types.Target{
			{
				Key:    aws.String("tag:Locust"),
				Values: []string{"true"},
			},
		},
	}, optFns...)
}

// Stop Locust worker services.
func StopLocustWorkerCommand(ctx context.Context, client SSMRunCommand, optFns ...func(*ssm.Options)) (*ssm.SendCommandOutput, error) {
	fmt.Printf("Stopping workers...\n")
	return client.SendCommand(ctx, &ssm.SendCommandInput{
		DocumentName: aws.String("AWS-RunShellScript"),
		Comment:      aws.String("Stop Locust Worker"),
		Parameters: map[string][]string{
			"commands": {
				"sudo pkill screen",
			},
		},
		Targets: []types.Target{
			{
				Key:    aws.String("tag:LocustState"),
				Values: []string{"worker"},
			},
		},
	}, optFns...)
}

// Stop Locust master service.
func StopLocustMasterCommand(ctx context.Context, client SSMRunCommand, optFns ...func(*ssm.Options)) (*ssm.SendCommandOutput, error) {
	fmt.Println("Stopping Locust Master...")

	return client.SendCommand(ctx, &ssm.SendCommandInput{
		DocumentName: aws.String("AWS-RunShellScript"),
		Comment:      aws.String("Stop Locust Master"),
		Parameters: map[string][]string{
			"commands": {
				"sudo pkill screen",
			},
		},
		Targets: []types.Target{
			{
				Key:    aws.String("tag:LocustState"),
				Values: []string{"master"},
			},
		},
	}, optFns...)
}

func main() {
	path, err := os.Getwd()
	if err != nil {
		log.Println(err)
	}
	err = godotenv.Load(path + "/.env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(os.Getenv("REGION")))
	if err != nil {
		panic("AWS configuration error, " + err.Error())
	}
	client := ssm.NewFromConfig(cfg)

	app := &cli.App{
		Name:     "liac",
		Version:  "0.1.0",
		Compiled: time.Now(),
		Authors: []*cli.Author{
			{
				Name:  "Sefa Aydinelli",
				Email: "sefa.aydinelli@bestcloudfor.me",
			},
		},
		Usage:       "Locust IaC CLI Tool",
		Description: "Locust IaC CLI tool for AWS",
		Commands: []*cli.Command{
			{
				Name:        "script",
				Usage:       "Pull the Git repository for Locust scripts",
				Description: "Pull or repull Git repository for Locust scripts",
				Subcommands: []*cli.Command{
					&cli.Command{
						Name:        "pull",
						Usage:       "Pull the Git repository for Locust scripts",
						Description: "Pull the Git repository for Locust scripts",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "repo-url",
								Usage: "Git repository url",
							},
						},
						Action: func(ctx *cli.Context) error {
							// get flag value
							repoURL := ctx.String("repo-url")
							if repoURL == "" {
								// pull default repository from environment variable
								repoURL = os.Getenv("GIT_REPOSITORY")
								response, err := GitPullCommand(repoURL, ctx.Context, client)
								if err != nil {
									return err
								}
								fmt.Println(response)
								fmt.Println("Git repository pull command sent successfully.")
								return nil
							} else {
								// pull given repository with flag
								response, err := GitPullCommand(repoURL, ctx.Context, client)
								if err != nil {
									return err
								}
								fmt.Println(response)
								fmt.Println("Git repository pull command sent successfully with given repository.")
								return nil
							}
						},
					},
					&cli.Command{
						Name:        "repull",
						Usage:       "Repull the Git repository for Locust scripts",
						Description: "Repull the Git repository for Locust scripts",
						// Flags: []cli.Flag{
						// 	&cli.StringFlag{
						// 		Name:  "repo-url",
						// 		Usage: "Git repository url",
						// 	},
						// },
						Action: func(ctx *cli.Context) error {
							// // get flag value
							// repoURL := ctx.String("repo-url")
							// if repoURL == "" {
							// 	// pull default repository from environment variable
							// 	repoURL = os.Getenv("GIT_REPOSITORY")
							// 	response, err := RePullGitRepoCommand(ctx.Context, client)
							// 	if err != nil {
							// 		return err
							// 	}
							// 	fmt.Println(response)
							// 	return nil
							// } else {
							// 	fmt.Printf("repull command with repo-url: %s\n", repoURL)
							// }
							// return nil
							response, err := RePullGitRepoCommand(ctx.Context, client)
							if err != nil {
								return err
							}
							fmt.Println(response)
							fmt.Println("Git repository repull command sent successfully.")
							return nil
						},
					},
				},
			},
			{
				Name:        "start",
				Usage:       "Start Locust master/worker services.",
				Description: "Start Locust master/worker services.",
				Subcommands: []*cli.Command{
					&cli.Command{
						Name:  "master",
						Usage: "Start master Locust.",
						Action: func(ctx *cli.Context) error {
							response, err := RunLocustMasterCommand(ctx.Context, client)
							if err != nil {
								return err
							}
							fmt.Println(response)
							fmt.Println("Locust master service start command sent successfully.")
							return nil
						},
						Description: "Start master Locust.",
					},
					&cli.Command{
						Name:  "worker",
						Usage: "Start Locust workers",
						Action: func(ctx *cli.Context) error {
							response, err := RunLocustWorkerCommand(ctx.Context, client)
							if err != nil {
								return err
							}
							fmt.Println(response)
							fmt.Println("Locust worker services start command sent successfully.")
							return nil
						},
						Description: "Start Locust workers.",
					},
				},
			},
			{
				Name:        "stop",
				Usage:       "Stop Locust master/worker services.",
				Description: "stop Locust master/worker services.",
				Subcommands: []*cli.Command{
					&cli.Command{
						Name:  "master",
						Usage: "Stop master Locust.",
						Action: func(ctx *cli.Context) error {
							response, err := StopLocustMasterCommand(ctx.Context, client)
							if err != nil {
								return err
							}
							fmt.Println(response)
							fmt.Println("Locust master service stop command sent successfully.")
							return nil
						},
						Description: "Stop master Locust.",
					},
					&cli.Command{
						Name:  "worker",
						Usage: "Stop Locust workers",
						Action: func(ctx *cli.Context) error {
							response, err := StopLocustWorkerCommand(ctx.Context, client)
							if err != nil {
								return err
							}
							fmt.Println(response)
							fmt.Println("Locust worker services stop command sent successfully.")
							return nil
						},
						Description: "Stop Locust workers.",
					},
					&cli.Command{
						Name:  "all",
						Usage: "Stop all Locust services.",
						Action: func(ctx *cli.Context) error {
							_, err := StopLocustMasterCommand(ctx.Context, client)
							if err != nil {
								return err
							}
							_, err = StopLocustWorkerCommand(ctx.Context, client)
							if err != nil {
								return err
							}
							fmt.Println("Locust services stop commands sent successfully.")
							return nil
						},
						Description: "Stop all Locust services.",
					},
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

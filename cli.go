package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
)

import (
	"code.google.com/p/gopass"
	"github.com/djeebus/go-plex"
	"github.com/codegangsta/cli"
	"github.com/crackcomm/go-clitable"
//	"github.com/djeebus/go-castv2"
	"gopkg.in/yaml.v2"
	"log"
	"net/http"
	"time"
)

const ConfigFileName = "config.yaml"

func getUsername(context *cli.Context)(username string, err error) {
	username = context.String("username")
	if len(username) > 0 {
		return username, nil
	}

	stdInReader := bufio.NewReader(os.Stdin)
	fmt.Printf("Plex username: ")
	return stdInReader.ReadString('\n')
}

func getPassword(context *cli.Context) (password string, err error) {
	password = context.String("password")
	if len(password) > 0 {
		return password, nil
	}
	return gopass.GetPass("Plex password: ")
}

type validConnection struct {
	Device		*plex.PlexDevice
	Connection	*plex.PlexDeviceConnection
}

type Configuration struct {
	PlexToken		string	`yaml:"plex_token"`
	PlexUrl			string	`yaml:"plex_url"`
	Chromecast		string	`yaml:"chromecast"`
}

func check(e error, code int, prompt string) {
	if e != nil {
		fmt.Printf("%s: %s\n", prompt, e)
		os.Exit(code)
	}
}

var listPlexServersCommand = cli.Command{
	Name: "list",
	Usage: "List all plex servers",
	Flags: []cli.Flag {
		cli.StringFlag{
			Name: "username",
			Usage: "Plex username",
		},
		cli.StringFlag{
			Name: "password",
			Usage: "Plex password",
		},
		cli.StringFlag{
			Name: "timeout",
			Usage: "Timeout connecting to servers",
		},
	},
	Action: func (context *cli.Context) {
		timeout := context.Duration("timeout")

		username, err := getUsername(context)
		check(err, 1, "Failed to get username")

		password, err := getPassword(context)
		check(err, 2, "Failed to get password")

		fmt.Print("Signing in ... ")
		user, err := plex.SignIn(username, password)
		check(err, 3, "failed to sign in")
		fmt.Println("done")

		devices, err := user.Devices()
		check(err, 4, "failed to get devices")

		table := clitable.New([]string{"Server Name", "Username", "Url", "Status"})

		client := &http.Client{
			Timeout: timeout,
		}

		for _, device := range devices {
			for _, connection := range device.Connections {
				go func (c *plex.PlexDeviceConnection,
						 d *plex.PlexDevice) {
					user := d.SourceTitle
					if len(user) == 0 {
						user = username
					}

					var status string
					canConnect := c.Validate(client)
					if canConnect {
						status = "Up"
					}

					table.AddRow(map[string]interface{}{
						"Server Name": d.Name,
						"Username": user,
						"Url": c.Uri,
						"Status": status,
					})
				}(connection, device)
			}
		}

		select {
		case <- time.After(timeout):
			table.Print()
			return
		}
	},
}

var listChromecastsCommand = cli.Command{
	Name: "list",
	Usage: "Find chromecasts",
	Flags: []cli.Flag{
		cli.DurationFlag{
			Name: "timeout",
			Usage: "Wait for this many seconds to find chromecasts",
			Value: time.Second * 15,

		},
	},
	Action: func (context *cli.Context) {
		timeout := context.Duration("timeout")
		fmt.Printf("Searching for chromecasts for %s ... \n", timeout)
		chromecasts, err := GetChromecasts(timeout)
		check(err, 5, "failed to find chromecasts")

		table := clitable.New([]string{"Chromecast Name", "Address"})

		for _, chromecast := range chromecasts {
			table.AddRow(map[string]interface{}{
				"Chromecast Name": chromecast.Name,
				"Address": fmt.Sprintf("%s:%d", chromecast.Addr, chromecast.Port),
			})
		}

		table.Print()
	},
}

var plexCommand = cli.Command{
	Name: "plex",
	Usage: "Plex commands",
	Subcommands: []cli.Command{
		listPlexServersCommand,
		plexTokenCommand,
	},
}

var chromecastCommand = cli.Command{
	Name: "chromecast",
	Usage: "Chromecast commands",
	Subcommands: [] cli.Command{
		listChromecastsCommand,
	},
}

var plexTokenCommand = cli.Command{
	Name: "token",
	Usage: "Get plex token",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name: "username",
			Usage: "Plex username",
		},
		cli.StringFlag{
			Name: "password",
			Usage: "Plex password",
		},
	},
	Action: func (context *cli.Context) {
		username, err := getUsername(context)
		check(err, 1, "Failed to get username")

		password, err := getPassword(context)
		check(err, 2, "Failed to get password")

		user, err := plex.SignIn(username, password)
		check(err, 3, "failed to sign in")

		fmt.Println(user.AuthToken)
	},
}

var configureCommand = cli.Command{
	Name: "configure",
	Usage: "Store credentials for future use",
	Flags: []cli.Flag {
		cli.StringFlag{
			Name: "plex-token",
			Usage: "Plex token",
		},
		cli.StringFlag{
			Name: "plex-url",
			Usage: "Plex URL",
		},
		cli.StringFlag{
			Name: "chromecast",
			Usage: "Chromecast host and port",
		},
	},
	Action: func (context *cli.Context) {
		token := context.String("plex-token")
		plexUrl := context.String("plex-url")
		chromecast := context.String("chromecast")

		config := Configuration{
			PlexToken: token,
			PlexUrl: plexUrl,
			Chromecast: chromecast,
		}

		data, err := yaml.Marshal(&config)
		check(err, 6, "Failed to create config")

		os.Remove(ConfigFileName)

		err = ioutil.WriteFile(ConfigFileName, data, 0664)
		check(err, 7, "Failed to write config")

		fmt.Printf("Done!!!!\n")
	},
}

type NullWriter int
func (NullWriter) Write([]byte) (int, error) {
	return 0, nil
}

func main() {
	log.SetOutput(new(NullWriter))

	app := cli.NewApp()
	app.Name = "goplexcast"
	app.Usage = "Launch a plex stream on chromecast"

	app.Commands = []cli.Command {
		plexCommand,
		chromecastCommand,
		configureCommand,
	}

	app.Run(os.Args)
}

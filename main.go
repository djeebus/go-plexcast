package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"time"
)

import (
	"code.google.com/p/gopass"
	"github.com/djeebus/goplex"
	"github.com/codegangsta/cli"
//	"github.com/djeebus/go-castv2"
	"github.com/turret-io/go-menu/menu"
	"gopkg.in/yaml.v2"
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

var ServerScanTimeout = 5 * time.Second

type validConnection struct {
	Device		*goplex.PlexDevice
	Connection	*goplex.PlexDeviceConnection
}

func validateDevice(device *goplex.PlexDevice, ch chan *validConnection) {
	cxns := make(chan *goplex.PlexDeviceConnection)

	for _, c := range device.Connections {
		go func () {
			result := c.Validate()
			if result {
				cxns <- c
			}
		} ()
	}

	timeout := time.After(ServerScanTimeout)
	for {
		select {
		case cxn := <-cxns:
			ch <- &validConnection{device, cxn}
			return
		case <- timeout:
			return
		}
	}
}

func getValidDevices(user *goplex.UserAuthQuery) ([]*validConnection, error) {
	devices, err := user.Devices()
	if err != nil {
		return nil, err
	}

	deviceCount := len(devices)
	validDevices := make([]*validConnection, 0, deviceCount)

	ch := make(chan bool)

	for _, d := range devices {
		go func (dev *goplex.PlexDevice) {
			cxn, err := dev.GetBestConnection(ServerScanTimeout)
			if err == nil {
				validDevices = append(validDevices, &validConnection{d, cxn})
			}
			ch <- true
		}(d)
	}

	for idx := 0; idx < deviceCount; idx ++ {
		<- ch
	}

	return validDevices, nil
}

type NoDevices struct {}
func (*NoDevices) Error() string {return "No devices found."}

func getDevice(user *goplex.UserAuthQuery) (*validConnection, error) {
	validDevices, err := getValidDevices(user)
	if err != nil {
		return nil, err
	}

	if len(validDevices) == 0 {
		return nil, &NoDevices{}
	} else if len(validDevices) == 1 {
		return validDevices[0], nil
	}

	fmt.Printf("Found %d valid devices\n", len(validDevices))

	var selectedDevice *validConnection
	commands := make([]menu.CommandOption, len(validDevices))

	for index, d := range validDevices {
		commands[index] = menu.CommandOption{
			fmt.Sprintf("%d", index + 1),
			d.Device.Name,
			func (cmd ...string) error {
				selectedDevice = d
				return nil
			},
		}
	}

	menuOptions := menu.NewMenuOptions("Select a server", 0)
	menu := menu.NewMenu(commands, menuOptions)
	menu.Start()

	return selectedDevice, nil
}

type Configuration struct {
	PlexToken		string	`yaml:"plex_token"`
	PlexUrl			string	`yaml:"plex_url"`
	ChromecastName	string	`yaml:"chromecast_name"`
}

func check(e error, code int, prompt string) {
	if e != nil {
		fmt.Printf("%s: %s\n", prompt, e)
		os.Exit(code)
	}
}

var configureCommand = cli.Command{
	Name: "configure",
	Usage: "Store credentials for future use",
	Flags: []cli.Flag {
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

		fmt.Print("Signing in ... ")
		user, err := goplex.SignIn(username, password)
		check(err, 3, "failed to sign in")
		fmt.Println("done")

		fmt.Print("Testing devices ... ")
		device, err := getDevice(user)
		check(err, 4, "failed to get device")
		fmt.Printf("got device: %s\n", device.Device.Name)

		fmt.Print("Discovering chromecasts ... ")
		chromecast, err := getChromecast()
		check(err, 5, "failed to find chromecasts")
		fmt.Printf("found %s\n", chromecast.Host)

		config := Configuration{
			PlexToken: user.AuthToken,
			PlexUrl: device.Connection.Uri,
			ChromecastName: chromecast.Name,
		}

		data, err := yaml.Marshal(&config)
		check(err, 6, "Failed to create config")

		os.Remove(ConfigFileName)

		err = ioutil.WriteFile(ConfigFileName, data, 0664)
		check(err, 7, "Failed to write config")

		fmt.Printf("Done!!!!\n")
	},
}

func getChromecast() (*ChromecastInfo, error) {
	chromecasts, err := GetChromecasts(1 * time.Minute)
	if err != nil {
		return nil, err
	}
	fmt.Println("done")

	if len(chromecasts) == 0 {
		return nil, &NoDevices{}
	} else if len(chromecasts) == 1 {
		return chromecasts[0], nil
	} else {
		fmt.Printf("Found %d chromecasts", len(chromecasts))
		os.Exit(6)
		return nil, nil
	}
}


func main() {
	app := cli.NewApp()
	app.Name = "PlexCast"
	app.Usage = "Launch a plex stream on chromecast"

	app.Commands = []cli.Command {
		configureCommand,
	}

	app.Run(os.Args)
}

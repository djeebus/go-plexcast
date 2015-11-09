package main

import (
	"fmt"
	"os"
	"bufio"
)

import (
	"code.google.com/p/gopass"
	"github.com/djeebus/goplex"
	"github.com/codegangsta/cli"
//	"github.com/djeebus/go-castv2"
	"github.com/turret-io/go-menu/menu"
	"time"
)

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
		go func () {
			cxn, err := d.GetBestConnection(ServerScanTimeout)
			if err == nil {
				validDevices = append(validDevices, &validConnection{d, cxn})
			}
			ch <- true
		}()
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
		if err != nil {
			fmt.Printf("Failed to get username: %s\n", err)
			os.Exit(1)
		}

		password, err := getPassword(context)
		if err != nil {
			fmt.Printf("Failed to get password: %s\n", err)
			os.Exit(2)
		}

		fmt.Print("Signing in ... \n")
		user, err := goplex.SignIn(username, password)
		if err != nil {
			fmt.Printf("Failed to sign in: %s\n", err)
			os.Exit(3)
			return
		}

		fmt.Print("Testing devices ... \n")
		device, err := getDevice(user)
		if err != nil {
			fmt.Printf("Failed to get device: %s\n", err)
			os.Exit(4)
			return
		}

		fmt.Printf("Got device: %s\n", device.Device.Name)
	},
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

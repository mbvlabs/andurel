package cli

import "fmt"

const bannerAndurel = `
    _    _   _ ____  _   _ ____  _____ _
   / \  | \ | |  _ \| | | |  _ \| ____| |
  / _ \ |  \| | | | | | | | |_) |  _| | |
 / ___ \| |\  | |_| | |_| |  _ <| |___| |___
/_/   \_\_| \_|____/ \___/|_| \_\_____|_____|
`

const bannerDoctor = `
 ____   ___   ____ _____ ___  ____
|  _ \ / _ \ / ___|_   _/ _ \|  _ \
| | | | | | | |     | || | | | |_) |
| |_| | |_| | |___  | || |_| |  _ <
|____/ \___/ \____| |_| \___/|_| \_\
`

func printBanner() {
	fmt.Print(bannerAndurel)
}

func printDoctorBanner() {
	fmt.Print(bannerAndurel)
	fmt.Print(bannerDoctor)
}

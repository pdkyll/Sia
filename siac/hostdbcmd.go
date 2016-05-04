package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/NebulousLabs/Sia/api"
	"github.com/NebulousLabs/Sia/types"
)

var (
	hostdbCmd = &cobra.Command{
		Use:   "hostdb",
		Short: "View or modify the host database",
		Long:  "Add and remove hosts, or list active hosts on the network.",
		Run:   wrap(hostdbcmd),
	}
)

func hostdbcmd() {
	info := new(api.ActiveHosts)
	err := getAPI("/renter/hosts/active", info)
	if err != nil {
		die("Could not fetch host list:", err)
	}
	if len(info.Hosts) == 0 {
		fmt.Println("No known active hosts")
		return
	}
	fmt.Println("Active hosts:")
	for _, host := range info.Hosts {
		price := host.StoragePrice.Mul(types.NewCurrency64(4320e12))
		fmt.Printf("\t%v - %v / TB / Month\n", host.NetAddress, currencyUnits(price))
	}
}

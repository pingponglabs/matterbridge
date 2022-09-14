# Appservice bridge setup

**- Copy  dendrite-config.yaml and registration.yaml to a directory**<br />
**- Run the dendrite server with dendrite-config as config file**<br />
**- Make sure the appservice port is the same on the registration.yaml file and appservice.toml file  it is default to "1234"**<br />
**- Edit appservice.toml  (change the MainUser field to your matrix user ID)**<br />
**- build and Run the matterbridge server with appservice.toml as config file**<br />
**- There should be the room named appservice-control created**<br />
**- Go to the room and make command "/appservice join #irc-channel-name" to join channels on irc**<br />

TODO
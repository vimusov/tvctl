# What?

A solution which allows you to rule your PC/laptop/mediabox/other by any IR remote control. It consists of two parts:

1. Arduino Nano with an IR sensor. It reads key codes from a remote control and sends them to a PC which Arduino is connected to.

1. `tvctl` - A daemon which receives key codes from Arduino and emulates keyboard actions according to a config file.

# Requirements

For running:

- [xdotool](https://github.com/jordansissel/xdotool);

For building:

- Go >= 1.19;
- [just](https://github.com/casey/just);
- [PlatformIO](https://platformio.org);

# Prepare hardware

1. Attach IR sensor (HX1838/TL1838/VS1838) to Arduino Nano:

   - Output to D2;
   - GND to GND;
   - Vcc to +5V;

1. Connect Arduino to a PC;

1. Build and flash the firwmare:

   `just flash`

# Prepare software

1. Build and install `tvctl` daemon:

   ```
   just install $DESTDIR
   ```
   Where `$DESTDIR` is the destination directory.

1. Copy config template:

   `cp /usr/share/tvctl/example.conf ~/.config/tvctl.conf`

1. Enable the daemon:

   `systemctl --user enable tvctl.service`

# Configure

1. Open the config `~/.config/tvctl.conf` in your favorite editor. Setup port acquired by Arduino and save the config.

1. Start the daemon in a debug mode:

   `tvctl -debug`

1. Edit the config pressing buttons on a remote control and mapping them to keyboard shortcuts.

1. Start the daemon in background mode:

   `systemctl --user start tvctl.service`

# License

GPL.

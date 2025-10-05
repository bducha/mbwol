# mbwol: Multi-Boot Wake on LAN

Wake your multi-boot computer and choose which OS to boot remotely !

`mbwol` allows you to power on a computer via Wake-on-LAN (WoL) and dynamically tell the **GRUB bootloader** which operating system to select, all through a simple HTTP request.

-----

## Features

  * **Remote OS Selection:** Choose your boot entry (e.g., Windows or Linux) from another device on your network.
  * **Wake-on-LAN Integration:** Powers on your machine and selects the OS in a single command.
  * **Dynamic GRUB Configuration:** Uses a built-in TFTP server to serve a temporary GRUB configuration.
  * **Flexible Setup:** Configurable timeouts and per-host settings via a simple JSON file.
  * **Lightweight and Simple:** No complex dependencies, designed to run 24/7 on a low-power device like a Raspberry Pi.

-----

## How It Works

The process is a sequence of simple network events, orchestrated by `mbwol`.

1.  **HTTP Request:** You send a request to the `mbwol` server, e.g., `GET /boot/desktop/windows`.
2.  **Wake-on-LAN:** The server immediately sends a "magic packet" to the target computer's MAC address, waking it up. It also temporarily stores your choice ("windows").
3.  **GRUB Boot:** The target computer powers on and boots into the GRUB bootloader.
4.  **TFTP Request:** As part of its startup script, GRUB sends a TFTP request to the `mbwol` server to fetch a configuration file.
5.  **Dynamic Response:** The `mbwol` server identifies the computer by its IP address and sends back the GRUB commands corresponding to your stored choice (e.g., `set default=1`).
6.  **OS Launch:** GRUB executes these commands, changing the default boot entry, and proceeds to boot into the selected OS.

-----

## Prerequisites

  * **Target Computer:** A multi-boot computer with **GRUB 2** as the bootloader. The motherboard and network card must support **Wake-on-LAN (WoL)**.
  * **Server:** A separate, ideally always-on, machine on the same local network to run the `mbwol` service (e.g., a home server, Raspberry Pi, etc.).

-----

## Running the service

### Docker

```bash
docker run -v ${PWD}/mbwol.json:/app/mbwol.json --net host ghcr.io/bducha/mbwol
```

When running with docker, make sure to use the network mode `host`, otherwise Wake on Lan won't work.

### Docker compose

See [docker-compose.yml](docker-compose.yml)

### Building from source

Requires at least Go 1.24

```bash
git pull https://github.com/bducha/mbwol.git
cd mbwol
go build .
sudo ./mbwol
```

Running mbwol requires root privileges to listen on port 69 (TFTP server port)

## Setup and Configuration

The setup involves two main parts: configuring the `mbwol` server and configuring the GRUB bootloader on your target computer.

### Part 1: `mbwol` Server Configuration

The application is configured using a single JSON file and can be customized with environment variables.

By default, the app looks for `mbwol.json` in the same directory as the executable. You can specify a different path using the **`MBWOL_CONFIG_FILE`** environment variable.

The web server listens on port **`8000`** by default. You can change this by setting the **`MBWOL_HTTP_PORT`** environment variable.

#### Configuration Options

| Key | Type | Description |
| :--- | :--- | :--- |
| `hosts` | Object | A map of all your target computers. The key is a unique ID for each host (e.g., "desktop"). |
| ↳ `ip` | String | The static IP address of the target computer. This is used to identify the host during a TFTP request. |
| ↳ `macAddress` | String | The MAC address of the target computer's network card, used for sending the WoL magic packet. |
| ↳ `broadcastIp` | String | (Optional) The broadcast address for the magic packet. Defaults to `255.255.255.255`. |
| ↳ `configs` | Object | A map of boot configurations. The key is the name you'll use in the URL (e.g., "windows"), and the value is the GRUB command(s) to be served. |
| ↳ `timeout` | Integer | Time in seconds after an HTTP request to keep a boot choice active. After this time, the choice is cleared. Set to `0` to disable the timeout. |
| ↳ `resetAfterGet` | Boolean | If `true`, the boot choice is cleared immediately after the target computer successfully requests it via TFTP. |

#### Example `mbwol.json`

```json
{
  "hosts": {
    "desktop": {
      "ip": "10.0.1.2",
      "macAddress": "00:1B:2C:3D:4E:5F",
      "broadcastIp": "255.255.255.255",
      "configs": {
        "arch": "set default=0\n",
        "windows": "set default=1\n"
      },
      "timeout": 120,
      "resetAfterGet": true
    }
  }
}
```

**Note:** The `\n` in the config values is important to ensure GRUB treats it as a valid line.

### Part 2: Target Computer Configuration (GRUB Client)

You need to configure your target computer to support WoL and to fetch its configuration from the `mbwol` server on boot.

#### Step 1: Enable Wake-on-LAN

This is done in two places:

1.  **BIOS/UEFI:** Look for a setting named "Wake on LAN", "Power On By PCI-E", or similar and enable it.
2.  **Operating Systems:** Each of your installed OSes may need to be configured to not fully power off the network card on shutdown. This process varies widely, so search online for "enable Wake on LAN" for your specific OS.

#### Step 2: Configure GRUB for Networking

Your GRUB bootloader needs network access.

1.  **Enable PXE/Network Boot:** In your BIOS/UEFI, ensure that the network stack is enabled. This may be part of an option like "Enable PXE booting".
2.  **Test Networking in GRUB:** Boot your PC and at the GRUB menu, press `c` to enter the command line.
    ```bash
    # Load network modules (efinet is for UEFI systems)
    insmod net
    insmod efinet

    # Check for your network card
    net_ls_cards
    # This should list your card, e.g., 'net0'

    # Get an IP address via DHCP
    net_bootp
    # Alternatively, set a static IP if DHCP fails:
    # net_add_addr net0 192.168.1.15

    # Test the TFTP connection to the mbwol server
    # Replace the IP with your mbwol server's IP
    cat (tftp,10.0.1.1)/config
    ```
    If `cat` returns nothing (and no errors), it's working\! `mbwol` serves an empty config by default. If you get an error, check your network settings and firewall rules.

#### Step 3: Make it Permanent

Now, add these commands to your main GRUB configuration so they run on every boot.

1.  Edit the custom configuration file, usually located at `/etc/grub.d/40_custom` (you'll need root permissions).

2.  Add the following lines. **Crucially, use `source` instead of `cat`** so GRUB executes the commands.

    ```bash
    # Add this to /etc/grub.d/40_custom

    insmod net
    insmod efinet
    insmod tftp
    net_bootp
    source (tftp,10.0.1.1)/config
    ```

    *(Remember to replace `10.0.1.1` with your `mbwol` server's IP address).*

3.  Update your GRUB configuration to apply the changes. The command depends on your Linux distribution:

      * **Debian/Ubuntu:** `sudo update-grub`
      * **Arch Linux/Fedora:** `sudo grub-mkconfig -o /boot/grub/grub.cfg`

Your computer is now ready\!

-----

## Usage Example

Let's use the example configuration from above. Your `mbwol` server is at `10.0.1.1`, and your desktop's GRUB menu has "Arch Linux" as the first option (index 0) and "Windows" as the second (index 1).

**Goal:** Wake the computer and boot into Windows.

**Action:** Make an HTTP POST request from any device on your network (e.g., your phone, laptop, or a script).

```bash
curl -X POST http://10.0.1.1:8000/boot/desktop/windows
```

*(Assuming `mbwol` is running on the default port `8000`).*

**Result:**

1.  Your desktop powers on.
2.  GRUB starts, runs the script from `40_custom`, and requests its config from `mbwol`.
3.  `mbwol` sends back `set default=1`.
4.  GRUB sets the default boot option to the second entry and automatically boots into Windows. 🎉

-----

## 💡 Troubleshooting

  * **PC doesn't wake up:**
      * Double-check that the MAC address in `mbwol.json` is correct.
      * Confirm WoL is enabled in both the BIOS and your operating systems. Some OSes disable it on shutdown by default.
      * Ensure the `mbwol` server is on the same subnet as the target PC. If not, you may need to configure your router to forward WoL packets.
  * **GRUB shows a TFTP error:**
      * Verify that the target PC can ping the `mbwol` server.
      * Check for firewalls on the server machine that might be blocking TFTP traffic (usually on UDP port 69).
      * Ensure the `ip` in `mbwol.json` matches the IP your computer gets in GRUB.

-----

## Contributing

Every contributions are welcome !

You can submit issues to suggest new features, or report bugs. 
If there is an issue that you want to work on, make sure to add a comment to inform everyone. You can then fork the repository and make a pull request when you're done.

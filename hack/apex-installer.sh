#!/bin/bash

OS="$(uname -s)"
info_message() {
    if [ -z "${1}" ]; then
        echo "info_message() requires a message"
        exit 1
    fi
    echo -e "[\033[0;34m ACTION \033[0m] ${1}"
}

pass_message() {
    if [ -z "${1}" ]; then
        echo "pass_message() requires a message"
        exit 1
    fi
    echo -e "[\033[0;32m PASSED \033[0m] ${1}"
}

error_message() {
    if [ -z "${1}" ]; then
        echo "error_message() requires a message"
        exit 1
    fi
    if [ -n "$1" ]; then
        echo -e "[\033[0;31m FAILED \033[0m] ] ${1}"
    fi
}

function setup() {
    if [ $OS != "Darwin" ] && [ $OS != "Linux" ]; then
        error_message "Currently supports Linux and Darwin."
        exit 1
    fi

    if [ $OS == "Darwin" ]; then
        if ! [ -x "$(command -v brew)" ]; then
            info_message "Brew is not installed. Installing Brew..."
            /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
            if [ "$?" == "0" ]; then 
                pass_message "Brew is installed successfully."
            else 
                error_message "Brew installation failed."
                exit 1
            fi
        fi
        if ! [ -x "$(command -v wg)" ]; then
            info_message "Wireguard is not installed. Installing WireGuard..."
            HOMEBREW_NO_AUTO_UPDATE=1 HOMEBREW_NO_INSTALL_CLEANUP=1 HOMEBREW_NO_ENV_HINTS=1 brew install wireguard-tools --quiet
            if [ "$?" == "0" ]; then 
                pass_message "WireGuard is installed successfully."
                wg --version
            else 
                error_message "WireGuard installation failed."
                exit 1
            fi
        fi
        info_message "Installing Apex..."
        sudo curl -fsSL https://jaywalking.s3.amazonaws.com/apex-amd64-darwin --output /usr/local/sbin/apex
        sudo chmod +x /usr/local/sbin/apex
        pass_message "Apex is installed successfully."
    fi

    if [ $OS == "Linux" ]; then
        . /etc/os-release
        linuxDistro="${NAME}"
        
        if ! [ -x "$(command -v wg)" ]; then
            info_message "Wireguard is not installed. Installing WireGuard..."
            if [ "$linuxDistro" == "Ubuntu" ]; then
                sudo DEBIAN_FRONTEND=noninteractive apt-get -qq --no-install-recommends install wireguard wireguard-tools -y
            elif [ "$linuxDistro" == "CentOS Stream" ] || [ "$linuxDistro" == "Fedora" ]; then
                sudo dnf -q install wireguard-tools -y
            else
                error_message "Currenly only support Ubuntu, Fedora and Centos Stream."
            fi

            if [ "$?" == "0" ]; then 
                pass_message "WireGuard is installed successfully."
                wg --version
            else 
                error_message "WireGuard installation failed."
                exit 1
            fi
        fi

        info_message "Installing Apex..."
        sudo curl -fsSL https://jaywalking.s3.amazonaws.com/apex-amd64-linux --output /usr/local/sbin/apex
        sudo chmod +x /usr/local/sbin/apex
        pass_message "Apex is installed successfully."

    fi
}

function cleanup() {
    if [ $OS == "Darwin" ]; then
        if [ -x "$(command -v wg)" ]; then
            info_message "Uninstalling WireGuard."
            HOMEBREW_NO_AUTO_UPDATE=1 HOMEBREW_NO_INSTALL_CLEANUP=1 HOMEBREW_NO_ENV_HINTS=1 brew remove wireguard-tools --quiet
            if [ "$?" == "0" ]; then 
                pass_message "WireGuard is uninstalled successfully."
            else 
                error_message "WireGuard uninstallation failed."
            fi
            sudo rm -f /usr/local/etc/wireguard/wg0-latest-rev.conf
            sudo rm -f /usr/local/etc/wireguard/wg0.conf
            sudo ifconfig wg0 down
        fi
            
        if [ -x "$(command -v brew)" ]; then
            info_message "Not uninstalling the Brew. If you would like to uninstall the brew please fire following commads"
            info_message 'echo | /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/uninstall.sh)"'
        fi

        sudo rm -f /usr/local/sbin/apex
        pass_message "Apex is uninstalled successfully."
    
    elif [ $OS == "Linux" ]; then
        . /etc/os-release
        linuxDistro="${NAME}"
        if [ -x "$(command -v wg)" ]; then
            info_message "Uninstalling WireGuard."
            if [ "$linuxDistro" == "Ubuntu" ]; then
                sudo DEBIAN_FRONTEND=noninteractive apt-get -qq purge wireguard wireguard-tools -y
            elif [ "$linuxDistro" == "CentOS Stream" ] || [ "$linuxDistro" == "Fedora" ]; then
                sudo dnf -q remove wireguard-tools -y
            else
                error_message "Currenly only support Ubuntu, Fedora and Centos Stream."
            fi
            if [ "$?" == "0" ]; then 
                pass_message "WireGuard is uninstalled successfully."
            else 
                error_message "WireGuard uninstallation failed."
            fi
            sudo rm -f /etc/wireguard/wg0-latest-rev.conf
            sudo rm -f /etc/wireguard/wg0.conf
            sudo ip link del wg0
        fi
        sudo rm -f /usr/local/sbin/apex
        pass_message "Apex is uninstalled successfully."
    fi
}

function help() {
    printf "\n"
    printf "Usage: %s [-iuh]\n" "$0"
    printf "\t-i Install apex and all required dependencies.\n"
    printf "\t-u Uninstall apex and it's dependencies. \n"
    printf "\t-h help\n"
    exit 1
}

while getopts "iuh" opt; do
    case $opt in
        i ) setup;;
        u ) cleanup;;
        h ) help
        exit 0;;
        *) help
        exit 1;;
    esac
done
if [ $# -eq 0 ]; then 
    help;exit 0 
fi

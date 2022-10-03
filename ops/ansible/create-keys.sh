#! /bin/bash
if [[ ! -e /etc/wireguard/public.key ]]; then
    wg genkey | sudo tee /etc/wireguard/private.key | wg pubkey | sudo tee /etc/wireguard/public.key
fi

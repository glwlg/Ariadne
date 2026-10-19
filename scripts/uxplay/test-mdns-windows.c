#include <stdio.h>
#include "lib/compat.h"

static IP_ADAPTER_ADDRESSES adapters[4];
static IP_ADAPTER_GATEWAY_ADDRESS gateway;
static IP_ADAPTER_UNICAST_ADDRESS unicast;
static struct sockaddr_in lan_address;

static ULONG WINAPI test_GetAdaptersAddresses(ULONG family, ULONG flags,
                                               PVOID reserved,
                                               PIP_ADAPTER_ADDRESSES output,
                                               PULONG size)
{
    (void) family;
    (void) flags;
    (void) reserved;
    (void) size;
    *output = adapters[0];
    return NO_ERROR;
}

#define GetAdaptersAddresses test_GetAdaptersAddresses
#include "lib/mdnsd/mdnsd.c"
#undef GetAdaptersAddresses

int main(void)
{
    WSADATA winsock;
    if (WSAStartup(MAKEWORD(2, 2), &winsock) != 0) {
        return 1;
    }
    lan_address.sin_family = AF_INET;
    lan_address.sin_addr.s_addr = inet_addr("192.168.1.11");
    unicast.Address.lpSockaddr = (struct sockaddr *) &lan_address;

    for (int i = 0; i < 4; i++) {
        adapters[i].OperStatus = IfOperStatusUp;
        adapters[i].IfType = IF_TYPE_ETHERNET_CSMACD;
        adapters[i].FirstUnicastAddress = &unicast;
        adapters[i].Description = L"Hyper-V Virtual Ethernet Adapter";
        if (i < 3) {
            adapters[i].Next = &adapters[i + 1];
        }
    }
    /* Ignore an internal switch without a gateway, a down adapter, and loopback. */
    adapters[1].OperStatus = IfOperStatusDown;
    adapters[1].FirstGatewayAddress = &gateway;
    adapters[2].IfType = IF_TYPE_SOFTWARE_LOOPBACK;
    adapters[2].FirstGatewayAddress = &gateway;
    adapters[3].FirstGatewayAddress = &gateway;

    if (mdns_get_default_ipv4() != lan_address.sin_addr.s_addr) {
        fprintf(stderr, "mDNS must select the Hyper-V LAN adapter with a gateway\n");
        return 1;
    }
    adapters[3].FirstGatewayAddress = NULL;
    if (mdns_get_default_ipv4() != 0) {
        fprintf(stderr, "mDNS must skip adapters without a usable gateway\n");
        return 1;
    }
    WSACleanup();
    puts("Windows mDNS adapter regression tests passed");
    return 0;
}

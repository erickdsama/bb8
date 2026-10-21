// bb8-eye: drives the WS2812 eye ring on GPIO12 (PWM0, DMA) with rpi_ws281x.
//
// bb8-head is Go without cgo, so the PWM/DMA part lives in this small helper.
// It needs root (/dev/mem). Two ways to run it:
//   - systemd socket activation (sistema/bb8-eye.socket + bb8-eye@.service):
//     root helper per connection, stdin = the socket; bb8-head connects to
//     /run/bb8-eye.sock as user bb8.
//   - by hand: sudo bb8-eye 16 < frames.bin
//
// Input: frames of 3*N bytes (R, G, B per LED, brightness already applied).
// On EOF or a signal it turns the ring off and exits.
//
// Build on the Zero (see sistema/instalar_zero.sh):
//   gcc -O2 -o bb8-eye bb8_eye.c rpi_ws281x/{ws2811,pwm,pcm,dma,mailbox,rpihw}.c -I rpi_ws281x -lm

#include <signal.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

#include "ws2811.h"

#define GPIO_PIN 12 // PWM0 (ALT0); on the ReSpeaker HAT: Grove digital port
// Not 10: on the Zero W with the ReSpeaker (WM8960) the kernel gives DMA 10 to I2S
// capture, and taking it kills arecord. Free hardware channels there: 12-14
// (see /sys/kernel/debug/dmaengine/summary and brcm,dma-channel-mask 0x7f35).
#define DMA_CHAN 13

static volatile sig_atomic_t running = 1;

static void stop(int sig) { (void)sig; running = 0; }

static int read_full(unsigned char *buf, size_t n) {
    size_t got = 0;
    while (got < n && running) {
        ssize_t r = read(STDIN_FILENO, buf + got, n - got);
        if (r <= 0) return 0;
        got += (size_t)r;
    }
    return got == n;
}

int main(int argc, char **argv) {
    int count = argc > 1 ? atoi(argv[1]) : 16;
    if (count < 1 || count > 256) {
        fprintf(stderr, "usage: bb8-eye [leds 1..256] < RGB frames\n");
        return 2;
    }
    ws2811_t ring;
    memset(&ring, 0, sizeof ring);
    ring.freq = WS2811_TARGET_FREQ;
    ring.dmanum = DMA_CHAN;
    ring.channel[0].gpionum = GPIO_PIN;
    ring.channel[0].count = count;
    ring.channel[0].strip_type = WS2811_STRIP_GRB;
    ring.channel[0].brightness = 255;

    struct sigaction sa;
    memset(&sa, 0, sizeof sa);
    sa.sa_handler = stop;
    sigaction(SIGINT, &sa, NULL);
    sigaction(SIGTERM, &sa, NULL);
    signal(SIGPIPE, SIG_IGN);

    ws2811_return_t ret = ws2811_init(&ring);
    if (ret != WS2811_SUCCESS) {
        fprintf(stderr, "bb8-eye: ws2811_init: %s\n", ws2811_get_return_t_str(ret));
        return 1;
    }
    unsigned char *frame = malloc((size_t)count * 3);
    while (running && read_full(frame, (size_t)count * 3)) {
        for (int i = 0; i < count; i++) {
            const unsigned char *p = frame + 3 * i;
            ring.channel[0].leds[i] = ((ws2811_led_t)p[0] << 16) | ((ws2811_led_t)p[1] << 8) | p[2];
        }
        if ((ret = ws2811_render(&ring)) != WS2811_SUCCESS) {
            fprintf(stderr, "bb8-eye: render: %s\n", ws2811_get_return_t_str(ret));
            break;
        }
    }
    for (int i = 0; i < count; i++) ring.channel[0].leds[i] = 0;
    ws2811_render(&ring);
    ws2811_fini(&ring);
    free(frame);
    return 0;
}

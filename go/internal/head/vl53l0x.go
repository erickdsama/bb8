package head

// Minimal VL53L0X time-of-flight driver over I2C (address 0x29), a port of the
// init sequence of Pololu's VL53L0X Arduino library: data init, SPAD setup,
// default tuning, reference calibration, then continuous back-to-back ranging.

import (
	"errors"
	"fmt"
	"time"

	"periph.io/x/conn/v3/i2c"
)

const (
	regSysRangeStart       = 0x00
	regSystemSequence      = 0x01
	regSystemIntConfigGPIO = 0x0A
	regSystemIntClear      = 0x0B
	regResultIntStatus     = 0x13
	regResultRangeStatus   = 0x14
	regMSRCConfigControl   = 0x60
	regFinalRateLimit      = 0x44
	regGPIOHVMuxActiveHigh = 0x84
	regSPADEnablesRef0     = 0xB0
	regRefEnStartSelect    = 0xB6
	regDynSPADNumRequested = 0x4E
	regDynSPADStartOffset  = 0x4F
	regModelID             = 0xC0
)

type VL53L0X struct {
	d    *i2c.Dev
	stop byte
}

func (v *VL53L0X) w(reg, val byte) error { return v.d.Tx([]byte{reg, val}, nil) }

func (v *VL53L0X) r(reg byte) (byte, error) {
	b := make([]byte, 1)
	err := v.d.Tx([]byte{reg}, b)
	return b[0], err
}

func (v *VL53L0X) r16(reg byte) (uint16, error) {
	b := make([]byte, 2)
	err := v.d.Tx([]byte{reg}, b)
	return uint16(b[0])<<8 | uint16(b[1]), err
}

// seq writes register/value pairs, stopping at the first error.
func (v *VL53L0X) seq(pairs ...byte) error {
	for i := 0; i+1 < len(pairs); i += 2 {
		if err := v.w(pairs[i], pairs[i+1]); err != nil {
			return err
		}
	}
	return nil
}

func (v *VL53L0X) waitBit(reg, mask byte, timeout time.Duration) (byte, error) {
	t0 := time.Now()
	for {
		b, err := v.r(reg)
		if err != nil {
			return 0, err
		}
		if b&mask != 0 {
			return b, nil
		}
		if time.Since(t0) > timeout {
			return 0, errors.New("vl53l0x: timeout")
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// NewVL53L0X initializes the sensor and starts continuous ranging.
func NewVL53L0X(bus i2c.Bus) (*VL53L0X, error) {
	v := &VL53L0X{d: &i2c.Dev{Bus: bus, Addr: 0x29}}
	if id, err := v.r(regModelID); err != nil || id != 0xEE {
		return nil, fmt.Errorf("vl53l0x not found (id %#x, %v)", id, err)
	}
	// I2C standard mode and the "stop variable".
	if err := v.seq(0x88, 0x00, 0x80, 0x01, 0xFF, 0x01, 0x00, 0x00); err != nil {
		return nil, err
	}
	stop, err := v.r(0x91)
	if err != nil {
		return nil, err
	}
	v.stop = stop
	if err := v.seq(0x00, 0x01, 0xFF, 0x00, 0x80, 0x00); err != nil {
		return nil, err
	}
	// Disable SIGNAL_RATE_MSRC and SIGNAL_RATE_PRE_RANGE limit checks.
	msrc, err := v.r(regMSRCConfigControl)
	if err != nil {
		return nil, err
	}
	// Signal rate limit 0.25 MCPS (Q9.7).
	if err := v.seq(regMSRCConfigControl, msrc|0x12); err != nil {
		return nil, err
	}
	if err := v.d.Tx([]byte{regFinalRateLimit, 0x00, 0x20}, nil); err != nil {
		return nil, err
	}
	if err := v.w(regSystemSequence, 0xFF); err != nil {
		return nil, err
	}
	if err := v.spads(); err != nil {
		return nil, err
	}
	if err := v.seq(tuning...); err != nil {
		return nil, err
	}
	// Interrupt on new sample ready, active low.
	if err := v.w(regSystemIntConfigGPIO, 0x04); err != nil {
		return nil, err
	}
	mux, err := v.r(regGPIOHVMuxActiveHigh)
	if err != nil {
		return nil, err
	}
	if err := v.seq(regGPIOHVMuxActiveHigh, mux&^0x10, regSystemIntClear, 0x01, regSystemSequence, 0xE8); err != nil {
		return nil, err
	}
	// Reference calibration: VHV then phase.
	for _, c := range [][2]byte{{0x01, 0x40}, {0x02, 0x00}} {
		if err := v.seq(regSystemSequence, c[0], regSysRangeStart, 0x01|c[1]); err != nil {
			return nil, err
		}
		if _, err := v.waitBit(regResultIntStatus, 0x07, 500*time.Millisecond); err != nil {
			return nil, err
		}
		if err := v.seq(regSystemIntClear, 0x01, regSysRangeStart, 0x00); err != nil {
			return nil, err
		}
	}
	if err := v.w(regSystemSequence, 0xE8); err != nil {
		return nil, err
	}
	// Continuous back-to-back.
	if err := v.seq(0x80, 0x01, 0xFF, 0x01, 0x00, 0x00, 0x91, v.stop, 0x00, 0x01, 0xFF, 0x00, 0x80, 0x00,
		regSysRangeStart, 0x02); err != nil {
		return nil, err
	}
	return v, nil
}

func (v *VL53L0X) spads() error {
	if err := v.seq(0x80, 0x01, 0xFF, 0x01, 0x00, 0x00, 0xFF, 0x06); err != nil {
		return err
	}
	x, err := v.r(0x83)
	if err != nil {
		return err
	}
	if err := v.seq(0x83, x|0x04, 0xFF, 0x07, 0x81, 0x01, 0x80, 0x01, 0x94, 0x6B, 0x83, 0x00); err != nil {
		return err
	}
	if _, err := v.waitBit(0x83, 0xFF, 500*time.Millisecond); err != nil {
		return err
	}
	if err := v.w(0x83, 0x01); err != nil {
		return err
	}
	tmp, err := v.r(0x92)
	if err != nil {
		return err
	}
	count, aperture := tmp&0x7F, tmp>>7&1 == 1
	if err := v.seq(0x81, 0x00, 0xFF, 0x06); err != nil {
		return err
	}
	if x, err = v.r(0x83); err != nil {
		return err
	}
	if err := v.seq(0x83, x&^0x04, 0xFF, 0x01, 0x00, 0x01, 0xFF, 0x00, 0x80, 0x00); err != nil {
		return err
	}
	ref := make([]byte, 6)
	if err := v.d.Tx([]byte{regSPADEnablesRef0}, ref); err != nil {
		return err
	}
	if err := v.seq(0xFF, 0x01, regDynSPADStartOffset, 0x00, regDynSPADNumRequested, 0x2C, 0xFF, 0x00,
		regRefEnStartSelect, 0xB4); err != nil {
		return err
	}
	first, enabled := 0, byte(0)
	if aperture {
		first = 12
	}
	for i := range 48 {
		if i < first || enabled == count {
			ref[i/8] &^= 1 << (i % 8)
		} else if ref[i/8]>>(i%8)&1 == 1 {
			enabled++
		}
	}
	return v.d.Tx(append([]byte{regSPADEnablesRef0}, ref...), nil)
}

// RangeMM reads one continuous sample (≤ ~50 ms). 0 = nothing in range.
func (v *VL53L0X) RangeMM() (int, error) {
	if _, err := v.waitBit(regResultIntStatus, 0x07, 100*time.Millisecond); err != nil {
		return 0, err
	}
	mm, err := v.r16(regResultRangeStatus + 10)
	if err != nil {
		return 0, err
	}
	if err := v.w(regSystemIntClear, 0x01); err != nil {
		return 0, err
	}
	if mm == 0 || mm >= 2000 { // 8190/8191 = nothing seen
		return 0, nil
	}
	return int(mm), nil
}

// tuning is ST's default tuning table (register, value).
var tuning = []byte{
	0xFF, 0x01, 0x00, 0x00, 0xFF, 0x00, 0x09, 0x00, 0x10, 0x00, 0x11, 0x00, 0x24, 0x01, 0x25, 0xFF,
	0x75, 0x00, 0xFF, 0x01, 0x4E, 0x2C, 0x48, 0x00, 0x30, 0x20, 0xFF, 0x00, 0x30, 0x09, 0x54, 0x00,
	0x31, 0x04, 0x32, 0x03, 0x40, 0x83, 0x46, 0x25, 0x60, 0x00, 0x27, 0x00, 0x50, 0x06, 0x51, 0x00,
	0x52, 0x96, 0x56, 0x08, 0x57, 0x30, 0x61, 0x00, 0x62, 0x00, 0x64, 0x00, 0x65, 0x00, 0x66, 0xA0,
	0xFF, 0x01, 0x22, 0x32, 0x47, 0x14, 0x49, 0xFF, 0x4A, 0x00, 0xFF, 0x00, 0x7A, 0x0A, 0x7B, 0x00,
	0x78, 0x21, 0xFF, 0x01, 0x23, 0x34, 0x42, 0x00, 0x44, 0xFF, 0x45, 0x26, 0x46, 0x05, 0x40, 0x40,
	0x0E, 0x06, 0x20, 0x1A, 0x43, 0x40, 0xFF, 0x00, 0x34, 0x03, 0x35, 0x44, 0xFF, 0x01, 0x31, 0x04,
	0x4B, 0x09, 0x4C, 0x05, 0x4D, 0x04, 0xFF, 0x00, 0x44, 0x00, 0x45, 0x20, 0x47, 0x08, 0x48, 0x28,
	0x67, 0x00, 0x70, 0x04, 0x71, 0x01, 0x72, 0xFE, 0x76, 0x00, 0x77, 0x00, 0xFF, 0x01, 0x0D, 0x01,
	0xFF, 0x00, 0x80, 0x01, 0x01, 0xF8, 0xFF, 0x01, 0x8E, 0x01, 0x00, 0x01, 0xFF, 0x00, 0x80, 0x00,
}

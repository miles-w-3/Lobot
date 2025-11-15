# LED Matrix Ticker

A programmable LED matrix display system for showing weather, sports scores, transit status, and more on a Raspberry Pi with RGB LED matrix panel.

## Features

- **Dual-mode development**: Develop on any machine with Pygame emulator, deploy to hardware
- **Clean abstraction layer**: Same code runs on both emulator and hardware
- **Easy configuration**: Auto-detects environment or manual mode selection
- **Multiple examples**: Color patterns, scrolling text, information displays
- **Poetry dependency management**: Clean separation of dev and hardware dependencies

## Hardware

### Recommended Setup
- **Display**: 64x32 RGB LED Matrix Panel
- **Controller**: Raspberry Pi (3/4/Zero 2 W)
- **HAT**: Adafruit RGB Matrix HAT or similar
- **Library**: [rpi-rgb-led-matrix](https://github.com/hzeller/rpi-rgb-led-matrix)

### Supported Configurations
- 64x32 (single panel)
- 64x64 (single panel, parallel chains)
- 128x32 (two panels chained)
- 128x64 (two panels chained + parallel)

## Quick Start

### 1. Installation

#### Development Machine (Emulator Mode)
```bash
cd led-matrix-ticker

# Install Poetry if you don't have it
curl -sSL https://install.python-poetry.org | python3 -

# Install dependencies with emulator support
poetry install --with emulator

# Activate virtual environment
poetry shell
```

#### Raspberry Pi (Hardware Mode)
```bash
cd led-matrix-ticker

# Install Poetry
curl -sSL https://install.python-poetry.org | python3 -

# Install base dependencies
poetry install --with hardware

# Install RGB LED matrix library (hardware only)
curl https://raw.githubusercontent.com/hzeller/rpi-rgb-led-matrix/master/bindings/python/install.sh | bash

# Activate virtual environment
poetry shell
```

### 2. Run Examples

#### Basic Color Patterns
```bash
python src/examples/demo_basic.py
```

Shows rainbow patterns, color bars, gradients, and animated effects.

#### Scrolling Text
```bash
python src/examples/demo_text.py
```

Demonstrates classic ticker-style scrolling text in various colors.

#### Information Display
```bash
python src/examples/demo_info_display.py
```

Rotating display showing time, weather, transit, sports, and news.

## Usage

### Auto-Detection
The system automatically detects whether to use emulator or hardware mode:

```python
from src.config import get_display

# Auto-detects based on platform (Linux ARM = hardware, others = emulator)
with get_display() as display:
    display.clear()
    display.set_pixel(0, 0, 255, 0, 0)  # Red pixel at top-left
    display.show()
```

### Manual Mode Selection
```python
from src.config import get_display

# Force emulator mode (useful for testing on Raspberry Pi)
display = get_display(mode='emulator')

# Force hardware mode
display = get_display(mode='hardware')

# Or use environment variable
# export DISPLAY_MODE=emulator
display = get_display()
```

### Using Presets
```python
from src.config import get_display_preset

# Standard 64x32 panel
display = get_display_preset('64x32')

# Larger 128x64 display
display = get_display_preset('128x64')

# Override brightness
display = get_display_preset('64x32', brightness=50)
```

### Basic Display Operations

```python
from src.config import get_display
from PIL import Image

with get_display() as display:
    # Clear display
    display.clear()

    # Set individual pixels
    display.set_pixel(x, y, r, g, b)

    # Fill entire display with color
    display.fill(255, 0, 0)  # Red

    # Display PIL Image
    img = Image.open('image.png')
    display.set_image(img, x_offset=0, y_offset=0)

    # Update display
    display.show()
```

### Creating Scrolling Text

```python
from src.config import get_display
from PIL import Image, ImageDraw, ImageFont
import time

with get_display() as display:
    # Create text image
    font = ImageFont.load_default()
    text_img = Image.new('RGB', (200, 32), color=(0, 0, 0))
    draw = ImageDraw.Draw(text_img)
    draw.text((0, 10), "Hello World!", font=font, fill=(255, 255, 255))

    # Scroll across display
    for offset in range(-200, display.width):
        display.clear()
        display.set_image(text_img, x_offset=offset, y_offset=0)
        display.show()
        time.sleep(0.03)
```

## Project Structure

```
led-matrix-ticker/
├── pyproject.toml              # Poetry configuration
├── README.md                   # This file
└── src/
    ├── __init__.py
    ├── display_backend.py      # Abstract base class + Pygame emulator
    ├── hardware_backend.py     # RGB LED matrix hardware wrapper
    ├── config.py               # Configuration and mode selection
    └── examples/
        ├── demo_basic.py       # Color patterns and animations
        ├── demo_text.py        # Scrolling text examples
        └── demo_info_display.py # Information ticker rotation
```

## Development Workflow

1. **Develop on your machine** using the Pygame emulator
   ```bash
   poetry install --with emulator
   poetry shell
   python src/examples/demo_basic.py
   ```

2. **Test your code** - Make changes, see results instantly in the emulator window

3. **Deploy to Raspberry Pi** - Copy code and run with hardware
   ```bash
   # On Raspberry Pi
   poetry install --with hardware
   # Install rpi-rgb-led-matrix library
   curl https://raw.githubusercontent.com/hzeller/rpi-rgb-led-matrix/master/bindings/python/install.sh | bash

   # Run on actual hardware (may need sudo for GPIO access)
   sudo $(which python) src/examples/demo_basic.py
   ```

## Configuration Options

### Emulator Options
```python
display = get_display(
    mode='emulator',
    width=64,
    height=32,
    pixel_size=10  # Size of each LED pixel in screen pixels
)
```

### Hardware Options
```python
display = get_display(
    mode='hardware',
    width=64,
    height=32,
    rows=32,           # Rows per panel (16 or 32)
    chain_length=1,    # Number of panels chained horizontally
    parallel=1,        # Number of parallel chains
    brightness=100,    # LED brightness (0-100)
    gpio_slowdown=4    # GPIO slowdown for stability (1-4)
)
```

## Next Steps

### Building Your Own Display

1. **Create a new Python file** for your display logic
2. **Import the config module** to get a display instance
3. **Implement your display logic** using the display API
4. **Test with emulator** during development
5. **Deploy to hardware** when ready

Example:
```python
from src.config import get_display
import time

with get_display() as display:
    while True:
        # Your display logic here
        display.clear()
        # ... draw content ...
        display.show()
        time.sleep(0.1)
```

### Integrating APIs

Add API integrations for live data:
- **Weather**: OpenWeatherMap, Weather.gov
- **Sports**: ESPN API, The Sports DB
- **Transit**: Local transit API (varies by city)
- **Stocks**: Alpha Vantage, Yahoo Finance
- **News**: NewsAPI, RSS feeds

### Ideas for Enhancement

- Weather forecasts with icons
- Live sports scores
- Transit arrival times
- Stock tickers
- Bitcoin/crypto prices
- Calendar events
- Social media mentions
- Server monitoring stats
- IoT sensor data

## Troubleshooting

### Emulator Issues

**Problem**: "pygame is required for emulator mode"
```bash
poetry install --with emulator
```

**Problem**: Emulator window not appearing
- Check that your system supports GUI windows
- Try setting `DISPLAY` environment variable on Linux

### Hardware Issues

**Problem**: "rpi-rgb-led-matrix library is required"
```bash
curl https://raw.githubusercontent.com/hzeller/rpi-rgb-led-matrix/master/bindings/python/install.sh | bash
```

**Problem**: "Permission denied" errors
```bash
# Run with sudo to access GPIO
sudo $(which python) your_script.py
```

**Problem**: Display flickering or artifacts
- Adjust `gpio_slowdown` parameter (try values 1-4)
- Check power supply (LED matrices need significant power)
- Verify ribbon cable connections

## Hardware Purchasing Guide

### LED Matrix Panel
- **Size**: 64x32 pixels (HUB75 interface)
- **Pitch**: 3mm-5mm recommended
- **Where**: Adafruit, Amazon, AliExpress
- **Cost**: $30-60

### RGB Matrix HAT
- **Option 1**: Adafruit RGB Matrix HAT (~$15)
- **Option 2**: Electrodragon RGB Matrix HAT (~$10)
- **Features**: Level shifters, power connectors, easy Pi mounting

### Raspberry Pi
- **Pi 3/4**: Best performance, more expensive
- **Pi Zero 2 W**: Budget option, slightly slower
- **Not recommended**: Original Pi Zero (too slow)

### Power Supply
- **Voltage**: 5V DC
- **Current**: 2-4A for single 64x32 panel
- **Important**: LED matrices draw significant power at full brightness

### Total Budget
- **Basic setup**: ~$80-100 (Pi + panel + HAT + power)
- **Enhanced setup**: ~$150-200 (better Pi, larger panels, accessories)

## License

This project is provided as-is for educational and personal use.

## Acknowledgments

- [rpi-rgb-led-matrix](https://github.com/hzeller/rpi-rgb-led-matrix) by Henner Zeller
- [Pygame](https://www.pygame.org/) for emulation support
- [Pillow](https://python-pillow.org/) for image processing

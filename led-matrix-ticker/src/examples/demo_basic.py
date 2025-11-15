#!/usr/bin/env python3
"""
Basic Demo - Color and Pattern Tests

Demonstrates basic display functionality with color patterns and simple animations.
"""

import time
import sys
import os

# Add parent directory to path for imports
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '../..'))

from src.config import get_display


def rainbow_pattern(display, offset=0):
    """Draw a rainbow pattern across the display."""
    for x in range(display.width):
        # Calculate hue based on position and offset
        hue = ((x + offset) % display.width) / display.width

        # Simple HSV to RGB conversion (S=1, V=1)
        h = hue * 6
        c = 255
        x_val = int(c * (1 - abs(h % 2 - 1)))

        if h < 1:
            r, g, b = c, x_val, 0
        elif h < 2:
            r, g, b = x_val, c, 0
        elif h < 3:
            r, g, b = 0, c, x_val
        elif h < 4:
            r, g, b = 0, x_val, c
        elif h < 5:
            r, g, b = x_val, 0, c
        else:
            r, g, b = c, 0, x_val

        # Fill column with this color
        for y in range(display.height):
            display.set_pixel(x, y, r, g, b)


def color_bars(display):
    """Draw vertical color bars."""
    colors = [
        (255, 0, 0),    # Red
        (0, 255, 0),    # Green
        (0, 0, 255),    # Blue
        (255, 255, 0),  # Yellow
        (255, 0, 255),  # Magenta
        (0, 255, 255),  # Cyan
        (255, 255, 255),# White
    ]

    bar_width = display.width // len(colors)

    for i, (r, g, b) in enumerate(colors):
        x_start = i * bar_width
        x_end = min((i + 1) * bar_width, display.width)

        for x in range(x_start, x_end):
            for y in range(display.height):
                display.set_pixel(x, y, r, g, b)


def checkerboard(display, size=4, offset=0):
    """Draw a checkerboard pattern."""
    for y in range(display.height):
        for x in range(display.width):
            # Calculate checkerboard cell
            cell_x = (x + offset) // size
            cell_y = y // size

            # Alternate colors
            if (cell_x + cell_y) % 2 == 0:
                r, g, b = 255, 0, 255  # Magenta
            else:
                r, g, b = 0, 255, 255  # Cyan

            display.set_pixel(x, y, r, g, b)


def gradient(display, horizontal=True):
    """Draw a color gradient."""
    for y in range(display.height):
        for x in range(display.width):
            if horizontal:
                intensity = int((x / display.width) * 255)
                r, g, b = intensity, 0, 255 - intensity
            else:
                intensity = int((y / display.height) * 255)
                r, g, b = 0, intensity, 255 - intensity

            display.set_pixel(x, y, r, g, b)


def run_demo():
    """Run the basic demonstration."""
    print("LED Matrix Ticker - Basic Demo")
    print("=" * 50)
    print()
    print("Starting display test...")
    print("Press Ctrl+C to exit")
    print()

    # Get display (auto-detects emulator vs hardware)
    with get_display() as display:
        print(f"Display: {display.width}x{display.height}")
        print(f"Backend: {type(display).__name__}")
        print()

        try:
            # Test 1: Color bars
            print("Test 1: Color bars")
            display.clear()
            color_bars(display)
            display.show()
            time.sleep(3)

            # Test 2: Horizontal gradient
            print("Test 2: Horizontal gradient")
            display.clear()
            gradient(display, horizontal=True)
            display.show()
            time.sleep(3)

            # Test 3: Vertical gradient
            print("Test 3: Vertical gradient")
            display.clear()
            gradient(display, horizontal=False)
            display.show()
            time.sleep(3)

            # Test 4: Animated checkerboard
            print("Test 4: Animated checkerboard (5 seconds)")
            for i in range(50):
                display.clear()
                checkerboard(display, size=4, offset=i)
                display.show()
                time.sleep(0.1)

            # Test 5: Animated rainbow
            print("Test 5: Animated rainbow (5 seconds)")
            for i in range(64):
                display.clear()
                rainbow_pattern(display, offset=i)
                display.show()
                time.sleep(0.08)

            # Final: Solid colors sequence
            print("Test 6: Solid color sequence")
            colors = [
                (255, 0, 0),    # Red
                (0, 255, 0),    # Green
                (0, 0, 255),    # Blue
                (255, 255, 0),  # Yellow
                (0, 0, 0),      # Black (off)
            ]

            for r, g, b in colors:
                display.clear()
                display.fill(r, g, b)
                display.show()
                time.sleep(1)

            print()
            print("Demo complete!")

        except KeyboardInterrupt:
            print("\nDemo interrupted by user")
        except Exception as e:
            print(f"\nError: {e}")
            import traceback
            traceback.print_exc()


if __name__ == '__main__':
    run_demo()

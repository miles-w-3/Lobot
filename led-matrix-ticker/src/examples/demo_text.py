#!/usr/bin/env python3
"""
Scrolling Text Demo

Demonstrates text rendering and scrolling on the LED matrix display.
Classic ticker-style text display.
"""

import time
import sys
import os
from PIL import Image, ImageDraw, ImageFont

# Add parent directory to path for imports
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '../..'))

from src.config import get_display


class TextScroller:
    """Helper class for scrolling text on LED matrix."""

    def __init__(self, display, text, color=(255, 255, 255), speed=1):
        """
        Initialize text scroller.

        Args:
            display: Display backend instance
            text: Text to scroll
            color: RGB color tuple (default: white)
            speed: Scroll speed in pixels per frame (default: 1)
        """
        self.display = display
        self.text = text
        self.color = color
        self.speed = speed
        self.position = display.width

        # Try to load a font, fall back to default if not available
        try:
            # Try to load a small bitmap font
            self.font = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 8)
        except:
            try:
                # Try alternative font path (common on different systems)
                self.font = ImageFont.truetype("/System/Library/Fonts/Helvetica.ttc", 8)
            except:
                # Fall back to default font
                self.font = ImageFont.load_default()

        # Create text image
        self._create_text_image()

    def _create_text_image(self):
        """Create an image of the text to scroll."""
        # Create a temporary image to measure text size
        temp_img = Image.new('RGB', (1, 1))
        temp_draw = ImageDraw.Draw(temp_img)

        # Get text bounding box
        bbox = temp_draw.textbbox((0, 0), self.text, font=self.font)
        text_width = bbox[2] - bbox[0]
        text_height = bbox[3] - bbox[1]

        # Create image for text (add some padding)
        self.text_width = text_width + 4
        text_image = Image.new('RGB', (self.text_width, self.display.height), color=(0, 0, 0))
        draw = ImageDraw.Draw(text_image)

        # Draw text centered vertically
        y_pos = (self.display.height - text_height) // 2
        draw.text((2, y_pos), self.text, font=self.font, fill=self.color)

        self.text_image = text_image

    def update(self):
        """Update scroll position and return True if text is still visible."""
        # Clear display
        self.display.clear()

        # Draw text at current position
        self.display.set_image(self.text_image, x_offset=int(self.position), y_offset=0)

        # Update position
        self.position -= self.speed

        # Check if text has scrolled completely off screen
        if self.position < -self.text_width:
            return False

        return True

    def reset(self):
        """Reset scroll position to start."""
        self.position = self.display.width


def run_demo():
    """Run the text scrolling demonstration."""
    print("LED Matrix Ticker - Text Scrolling Demo")
    print("=" * 50)
    print()
    print("Starting scrolling text display...")
    print("Press Ctrl+C to exit")
    print()

    # Get display (auto-detects emulator vs hardware)
    with get_display() as display:
        print(f"Display: {display.width}x{display.height}")
        print(f"Backend: {type(display).__name__}")
        print()

        try:
            # Demo 1: Simple white text
            print("Demo 1: Simple scrolling text")
            scroller = TextScroller(
                display,
                "Hello, LED Matrix!",
                color=(255, 255, 255),
                speed=1
            )

            while scroller.update():
                display.show()
                time.sleep(0.03)

            time.sleep(0.5)

            # Demo 2: Colored text
            messages = [
                ("Weather: Sunny, 72°F", (255, 255, 0)),    # Yellow
                ("Traffic: Light delays", (0, 255, 0)),     # Green
                ("Time: 3:45 PM", (0, 255, 255)),           # Cyan
                ("Breaking News!", (255, 0, 0)),            # Red
            ]

            print("Demo 2: Multi-colored ticker messages")
            for text, color in messages * 2:  # Show each message twice
                scroller = TextScroller(display, text, color=color, speed=1)

                while scroller.update():
                    display.show()
                    time.sleep(0.03)

                time.sleep(0.3)

            # Demo 3: Fast scroll
            print("Demo 3: Fast scroll")
            scroller = TextScroller(
                display,
                "This is scrolling FAST! " * 3,
                color=(255, 128, 0),  # Orange
                speed=2
            )

            while scroller.update():
                display.show()
                time.sleep(0.02)

            print()
            print("Demo complete!")

        except KeyboardInterrupt:
            print("\nDemo interrupted by user")
        except Exception as e:
            print(f"\nError: {e}")
            import traceback
            traceback.print_exc()
        finally:
            display.clear()
            display.show()


if __name__ == '__main__':
    run_demo()

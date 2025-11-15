#!/usr/bin/env python3
"""
Information Display Demo

Demonstrates a more practical use case: displaying multiple pieces of
information (time, weather, etc.) in a rotating ticker format.
"""

import time
import sys
import os
from datetime import datetime
from PIL import Image, ImageDraw, ImageFont

# Add parent directory to path for imports
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '../..'))

from src.config import get_display


class InfoDisplay:
    """Multi-line information display manager."""

    def __init__(self, display):
        """Initialize info display."""
        self.display = display

        # Try to load fonts
        try:
            self.font = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 8)
        except:
            try:
                self.font = ImageFont.truetype("/System/Library/Fonts/Helvetica.ttc", 8)
            except:
                self.font = ImageFont.load_default()

    def draw_text(self, text, y_position, color=(255, 255, 255), center=False):
        """
        Draw text at a specific vertical position.

        Args:
            text: Text to draw
            y_position: Vertical position (0 to height-1)
            color: RGB color tuple
            center: If True, center text horizontally
        """
        # Create image for text
        img = Image.new('RGB', (self.display.width, 10), color=(0, 0, 0))
        draw = ImageDraw.Draw(img)

        # Get text size
        bbox = draw.textbbox((0, 0), text, font=self.font)
        text_width = bbox[2] - bbox[0]

        # Calculate x position
        if center:
            x_pos = max(0, (self.display.width - text_width) // 2)
        else:
            x_pos = 0

        # Draw text
        draw.text((x_pos, 0), text, font=self.font, fill=color)

        # Set pixels on display
        self.display.set_image(img, x_offset=0, y_offset=y_position)

    def show_time(self):
        """Display current time."""
        now = datetime.now()
        time_str = now.strftime("%I:%M %p")
        date_str = now.strftime("%m/%d/%y")

        self.display.clear()
        self.draw_text(time_str, 2, color=(0, 255, 255), center=True)
        self.draw_text(date_str, 12, color=(255, 255, 0), center=True)
        self.draw_text(now.strftime("%A")[:3].upper(), 22, color=(128, 128, 128), center=True)
        self.display.show()

    def show_weather(self, temp, condition):
        """
        Display weather information.

        Args:
            temp: Temperature string (e.g., "72°F")
            condition: Weather condition (e.g., "Sunny")
        """
        self.display.clear()
        self.draw_text("WEATHER", 0, color=(100, 100, 100), center=True)
        self.draw_text(temp, 10, color=(255, 165, 0), center=True)
        self.draw_text(condition, 20, color=(0, 255, 255), center=True)
        self.display.show()

    def show_message(self, line1, line2="", line3="", colors=None):
        """
        Display a multi-line message.

        Args:
            line1: First line of text
            line2: Second line of text (optional)
            line3: Third line of text (optional)
            colors: List of color tuples for each line
        """
        if colors is None:
            colors = [(255, 255, 255)] * 3

        self.display.clear()

        y_positions = [2, 12, 22] if line3 else [8, 18]

        lines = [line1, line2, line3] if line3 else [line1, line2]
        lines = [l for l in lines if l]  # Filter empty lines

        for i, (line, y_pos) in enumerate(zip(lines, y_positions)):
            color = colors[i] if i < len(colors) else (255, 255, 255)
            self.draw_text(line, y_pos, color=color, center=True)

        self.display.show()


def run_demo():
    """Run the information display demonstration."""
    print("LED Matrix Ticker - Information Display Demo")
    print("=" * 50)
    print()
    print("Starting info display rotation...")
    print("Press Ctrl+C to exit")
    print()

    # Get display (auto-detects emulator vs hardware)
    with get_display() as display:
        print(f"Display: {display.width}x{display.height}")
        print(f"Backend: {type(display).__name__}")
        print()

        info = InfoDisplay(display)

        try:
            rotation_count = 0

            while True:
                rotation_count += 1
                print(f"Rotation {rotation_count}")

                # Show time
                print("  - Showing time...")
                info.show_time()
                time.sleep(3)

                # Show weather (simulated data)
                print("  - Showing weather...")
                info.show_weather("72°F", "Sunny")
                time.sleep(3)

                # Show transit info (simulated)
                print("  - Showing transit...")
                info.show_message(
                    "TRANSIT",
                    "Bus 42",
                    "5 mins",
                    colors=[(100, 100, 100), (0, 255, 0), (255, 255, 255)]
                )
                time.sleep(3)

                # Show sports score (simulated)
                print("  - Showing sports...")
                info.show_message(
                    "SCORE",
                    "HOME 3",
                    "AWAY 2",
                    colors=[(100, 100, 100), (0, 255, 0), (255, 0, 0)]
                )
                time.sleep(3)

                # Show news alert (simulated)
                print("  - Showing news...")
                info.show_message(
                    "NEWS",
                    "Market",
                    "Up 2%",
                    colors=[(255, 255, 0), (255, 255, 255), (0, 255, 0)]
                )
                time.sleep(3)

                print()

        except KeyboardInterrupt:
            print("\nDemo interrupted by user")
        except Exception as e:
            print(f"\nError: {e}")
            import traceback
            traceback.print_exc()
        finally:
            display.clear()
            display.show()

        print("Demo complete!")


if __name__ == '__main__':
    run_demo()

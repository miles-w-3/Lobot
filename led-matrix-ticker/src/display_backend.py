"""
Display Backend Abstraction Layer

Provides a common interface for both emulated (Pygame) and hardware (LED matrix) displays.
This allows development and testing on any machine before deploying to Raspberry Pi.
"""

from abc import ABC, abstractmethod
from typing import Tuple
from PIL import Image


class DisplayBackend(ABC):
    """Abstract base class for display backends."""

    def __init__(self, width: int = 64, height: int = 32):
        """
        Initialize display backend.

        Args:
            width: Display width in pixels (default: 64)
            height: Display height in pixels (default: 32)
        """
        self.width = width
        self.height = height

    @abstractmethod
    def set_pixel(self, x: int, y: int, r: int, g: int, b: int) -> None:
        """
        Set a single pixel to the specified RGB color.

        Args:
            x: X coordinate (0 to width-1)
            y: Y coordinate (0 to height-1)
            r: Red value (0-255)
            g: Green value (0-255)
            b: Blue value (0-255)
        """
        pass

    @abstractmethod
    def clear(self) -> None:
        """Clear the entire display (set all pixels to black)."""
        pass

    @abstractmethod
    def show(self) -> None:
        """Update the display with the current buffer."""
        pass

    @abstractmethod
    def cleanup(self) -> None:
        """Clean up resources when done."""
        pass

    def set_image(self, image: Image.Image, x_offset: int = 0, y_offset: int = 0) -> None:
        """
        Set pixels from a PIL Image.

        Args:
            image: PIL Image to display (will be converted to RGB)
            x_offset: X offset for image placement (default: 0)
            y_offset: Y offset for image placement (default: 0)
        """
        # Convert image to RGB if needed
        if image.mode != 'RGB':
            image = image.convert('RGB')

        # Get image dimensions
        img_width, img_height = image.size

        # Set pixels from image
        for y in range(img_height):
            for x in range(img_width):
                # Calculate display coordinates
                display_x = x + x_offset
                display_y = y + y_offset

                # Skip if outside display bounds
                if display_x < 0 or display_x >= self.width:
                    continue
                if display_y < 0 or display_y >= self.height:
                    continue

                # Get pixel color and set it
                r, g, b = image.getpixel((x, y))
                self.set_pixel(display_x, display_y, r, g, b)

    def fill(self, r: int, g: int, b: int) -> None:
        """
        Fill the entire display with a single color.

        Args:
            r: Red value (0-255)
            g: Green value (0-255)
            b: Blue value (0-255)
        """
        for y in range(self.height):
            for x in range(self.width):
                self.set_pixel(x, y, r, g, b)


class PygameEmulator(DisplayBackend):
    """
    Pygame-based emulator for LED matrix display.

    Creates a window that mimics a 64x32 LED matrix display.
    Ideal for development and testing before hardware deployment.
    """

    def __init__(self, width: int = 64, height: int = 32, pixel_size: int = 10):
        """
        Initialize Pygame emulator.

        Args:
            width: Display width in pixels (default: 64)
            height: Display height in pixels (default: 32)
            pixel_size: Size of each emulated pixel in screen pixels (default: 10)
        """
        super().__init__(width, height)

        try:
            import pygame
        except ImportError:
            raise ImportError(
                "Pygame is required for emulator mode.\n"
                "Install with: poetry install --with emulator"
            )

        self.pygame = pygame
        self.pixel_size = pixel_size

        # Initialize Pygame
        pygame.init()

        # Create window
        window_width = width * pixel_size
        window_height = height * pixel_size
        self.screen = pygame.display.set_mode((window_width, window_height))
        pygame.display.set_caption(f"LED Matrix Emulator ({width}x{height})")

        # Create pixel buffer
        self.buffer = [[(0, 0, 0) for _ in range(width)] for _ in range(height)]

        # Clear display
        self.clear()

    def set_pixel(self, x: int, y: int, r: int, g: int, b: int) -> None:
        """Set a single pixel in the buffer."""
        if 0 <= x < self.width and 0 <= y < self.height:
            self.buffer[y][x] = (r, g, b)

    def clear(self) -> None:
        """Clear the display buffer."""
        self.buffer = [[(0, 0, 0) for _ in range(self.width)] for _ in range(self.height)]
        self.screen.fill((0, 0, 0))

    def show(self) -> None:
        """
        Update the display with the current buffer.

        Also handles Pygame events to keep window responsive.
        """
        # Handle Pygame events
        for event in self.pygame.event.get():
            if event.type == self.pygame.QUIT:
                self.cleanup()
                raise SystemExit("Emulator window closed")

        # Draw pixels
        for y in range(self.height):
            for x in range(self.width):
                color = self.buffer[y][x]
                rect = (
                    x * self.pixel_size,
                    y * self.pixel_size,
                    self.pixel_size,
                    self.pixel_size
                )
                self.pygame.draw.rect(self.screen, color, rect)

        # Update display
        self.pygame.display.flip()

    def cleanup(self) -> None:
        """Clean up Pygame resources."""
        self.pygame.quit()

    def __enter__(self):
        """Context manager entry."""
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        """Context manager exit."""
        self.cleanup()

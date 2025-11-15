"""
Hardware Backend for RGB LED Matrix

Wrapper around the rpi-rgb-led-matrix library for Raspberry Pi.
This module should only be imported on a Raspberry Pi with the library installed.

Installation on Raspberry Pi:
    curl https://raw.githubusercontent.com/hzeller/rpi-rgb-led-matrix/master/bindings/python/install.sh | bash
"""

from typing import Optional
from .display_backend import DisplayBackend


class RGBMatrixBackend(DisplayBackend):
    """
    Hardware backend for RGB LED matrix panels.

    Uses the rpi-rgb-led-matrix library to drive actual LED matrix hardware
    connected to a Raspberry Pi via RGB Matrix HAT.
    """

    def __init__(
        self,
        width: int = 64,
        height: int = 32,
        rows: int = 32,
        chain_length: int = 1,
        parallel: int = 1,
        brightness: int = 100,
        gpio_slowdown: int = 4,
        **kwargs
    ):
        """
        Initialize RGB LED matrix hardware.

        Args:
            width: Display width in pixels (default: 64)
            height: Display height in pixels (default: 32)
            rows: Number of rows per panel (16 or 32, default: 32)
            chain_length: Number of panels chained horizontally (default: 1)
            parallel: Number of parallel chains (default: 1)
            brightness: LED brightness 0-100 (default: 100)
            gpio_slowdown: GPIO slowdown for stability (1-4, default: 4)
            **kwargs: Additional options for RGBMatrixOptions
        """
        super().__init__(width, height)

        try:
            from rgbmatrix import RGBMatrix, RGBMatrixOptions
        except ImportError:
            raise ImportError(
                "rpi-rgb-led-matrix library is required for hardware mode.\n"
                "Install on Raspberry Pi with:\n"
                "curl https://raw.githubusercontent.com/hzeller/rpi-rgb-led-matrix/"
                "master/bindings/python/install.sh | bash"
            )

        # Configure matrix options
        options = RGBMatrixOptions()
        options.rows = rows
        options.cols = width // chain_length
        options.chain_length = chain_length
        options.parallel = parallel
        options.brightness = brightness
        options.gpio_slowdown = gpio_slowdown

        # Apply additional options
        for key, value in kwargs.items():
            if hasattr(options, key):
                setattr(options, key, value)

        # Initialize matrix
        self.matrix = RGBMatrix(options=options)

        # Get canvas for double buffering
        self.canvas = self.matrix.CreateFrameCanvas()

    def set_pixel(self, x: int, y: int, r: int, g: int, b: int) -> None:
        """Set a single pixel on the LED matrix."""
        if 0 <= x < self.width and 0 <= y < self.height:
            self.canvas.SetPixel(x, y, r, g, b)

    def clear(self) -> None:
        """Clear the LED matrix."""
        self.canvas.Clear()

    def show(self) -> None:
        """Update the LED matrix display (swap buffers)."""
        self.canvas = self.matrix.SwapOnVSync(self.canvas)

    def cleanup(self) -> None:
        """Clean up hardware resources."""
        if hasattr(self, 'matrix'):
            self.matrix.Clear()

    def set_brightness(self, brightness: int) -> None:
        """
        Set display brightness.

        Args:
            brightness: Brightness level 0-100
        """
        self.matrix.brightness = max(0, min(100, brightness))

    def __enter__(self):
        """Context manager entry."""
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        """Context manager exit."""
        self.cleanup()

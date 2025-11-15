"""
Configuration Module

Provides easy configuration and backend selection for the LED matrix display.
Supports both emulated (Pygame) and hardware (RGB LED Matrix) backends.
"""

import os
import platform
from typing import Optional
from .display_backend import DisplayBackend, PygameEmulator


def get_display(
    mode: Optional[str] = None,
    width: int = 64,
    height: int = 32,
    **kwargs
) -> DisplayBackend:
    """
    Get a display backend based on mode and environment.

    Args:
        mode: Display mode - 'emulator', 'hardware', or None for auto-detect
        width: Display width in pixels (default: 64)
        height: Display height in pixels (default: 32)
        **kwargs: Additional backend-specific options

    Returns:
        DisplayBackend: Configured display backend

    Environment Variables:
        DISPLAY_MODE: Set to 'emulator' or 'hardware' to override auto-detection

    Examples:
        # Auto-detect mode
        display = get_display()

        # Force emulator mode
        display = get_display(mode='emulator')

        # Hardware mode with custom brightness
        display = get_display(mode='hardware', brightness=75)

        # Larger display
        display = get_display(width=128, height=64)
    """
    # Check environment variable if mode not specified
    if mode is None:
        mode = os.environ.get('DISPLAY_MODE', None)

    # Auto-detect if still not specified
    if mode is None:
        mode = auto_detect_mode()

    # Normalize mode
    mode = mode.lower()

    # Create appropriate backend
    if mode == 'emulator':
        return create_emulator_backend(width, height, **kwargs)
    elif mode == 'hardware':
        return create_hardware_backend(width, height, **kwargs)
    else:
        raise ValueError(
            f"Invalid display mode: {mode}\n"
            "Valid modes: 'emulator', 'hardware'"
        )


def auto_detect_mode() -> str:
    """
    Auto-detect the appropriate display mode.

    Returns 'hardware' if running on Linux ARM (Raspberry Pi),
    otherwise returns 'emulator' for development.

    Returns:
        str: 'emulator' or 'hardware'
    """
    # Check if running on Linux ARM (typical Raspberry Pi)
    system = platform.system()
    machine = platform.machine()

    # Raspberry Pi detection
    if system == 'Linux' and machine.startswith('arm'):
        # Additional check: see if we can import the hardware library
        try:
            import rgbmatrix
            return 'hardware'
        except ImportError:
            pass

    # Default to emulator for development
    return 'emulator'


def create_emulator_backend(width: int, height: int, **kwargs) -> PygameEmulator:
    """
    Create a Pygame emulator backend.

    Args:
        width: Display width in pixels
        height: Display height in pixels
        **kwargs: Additional options (pixel_size, etc.)

    Returns:
        PygameEmulator: Configured emulator backend
    """
    # Extract emulator-specific options
    pixel_size = kwargs.pop('pixel_size', 10)

    return PygameEmulator(
        width=width,
        height=height,
        pixel_size=pixel_size
    )


def create_hardware_backend(width: int, height: int, **kwargs):
    """
    Create an RGB LED matrix hardware backend.

    Args:
        width: Display width in pixels
        height: Display height in pixels
        **kwargs: Hardware-specific options (brightness, gpio_slowdown, etc.)

    Returns:
        RGBMatrixBackend: Configured hardware backend
    """
    from .hardware_backend import RGBMatrixBackend

    return RGBMatrixBackend(
        width=width,
        height=height,
        **kwargs
    )


# Display configuration presets
DISPLAY_PRESETS = {
    '64x32': {'width': 64, 'height': 32, 'rows': 32, 'chain_length': 1},
    '64x64': {'width': 64, 'height': 64, 'rows': 32, 'chain_length': 1, 'parallel': 2},
    '128x32': {'width': 128, 'height': 32, 'rows': 32, 'chain_length': 2},
    '128x64': {'width': 128, 'height': 64, 'rows': 32, 'chain_length': 2, 'parallel': 2},
}


def get_display_preset(preset: str, mode: Optional[str] = None, **kwargs) -> DisplayBackend:
    """
    Get a display backend using a preset configuration.

    Args:
        preset: Preset name ('64x32', '64x64', '128x32', '128x64')
        mode: Display mode - 'emulator', 'hardware', or None for auto-detect
        **kwargs: Additional options to override preset values

    Returns:
        DisplayBackend: Configured display backend

    Examples:
        # Standard 64x32 panel
        display = get_display_preset('64x32')

        # Larger 128x64 display
        display = get_display_preset('128x64')

        # Override brightness
        display = get_display_preset('64x32', brightness=50)
    """
    if preset not in DISPLAY_PRESETS:
        available = ', '.join(DISPLAY_PRESETS.keys())
        raise ValueError(
            f"Unknown preset: {preset}\n"
            f"Available presets: {available}"
        )

    # Merge preset with overrides
    config = {**DISPLAY_PRESETS[preset], **kwargs}

    return get_display(mode=mode, **config)

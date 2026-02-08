from typing import Any, Dict

from aenv import register_function, register_reward, register_tool


@register_tool
def get_weather(city: str) -> Dict[str, Any]:
    """Get weather information for a city.

    Args:
        city: Name of the city to get weather for

    Returns:
        Dictionary containing weather information
    """
    return {
        "city": city,
        "temperature": "20",
        "description": f"Weather in {city}",
        "humidity": "65%"
    }


@register_function
def get_weather_func(city: str) -> Dict[str, Any]:
    """Function version of weather retrieval.

    Args:
        city: Name of the city to get weather for

    Returns:
        Dictionary containing weather information
    """
    return {
        "city": city,
        "temperature": "20",
        "description": f"Weather in {city}",
        "humidity": "65%"
    }


@register_reward
def is_good_weather(city: str) -> bool:
    """Check if weather conditions are favorable.

    Args:
        city: Name of the city to check weather for

    Returns:
        Boolean indicating if weather is good (temperature between 15-30°C)
    """
    result = get_weather(city)
    temp = int(result["temperature"])
    return 15 < temp < 30

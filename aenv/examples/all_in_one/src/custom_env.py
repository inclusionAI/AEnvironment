from typing import Any, Dict

from aenv import register_function, register_reward, register_tool


@register_tool
def get_weather(city: str) -> Dict[str, Any]:
    return {"city": city, "temperature": "20", "description": city, "humidity": "conf"}


@register_function
def get_weather_func(city: str) -> Dict[str, Any]:
    return {"city": city, "temperature": "20", "description": city, "humidity": "conf"}


@register_reward
def is_good_weather(city: str) -> bool:
    result = get_weather(city)
    return int(result["temperature"]) > 15 and int(result["temperature"]) < 30

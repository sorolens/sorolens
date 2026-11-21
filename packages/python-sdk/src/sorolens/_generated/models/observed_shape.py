from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

T = TypeVar("T", bound="ObservedShape")


@_attrs_define
class ObservedShape:
    """
    Attributes:
        topic_arity (int):
        topic_types (list[str]):
        value_type (str):
        count (int):
    """

    topic_arity: int
    topic_types: list[str]
    value_type: str
    count: int
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        topic_arity = self.topic_arity

        topic_types = self.topic_types

        value_type = self.value_type

        count = self.count

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "topic_arity": topic_arity,
                "topic_types": topic_types,
                "value_type": value_type,
                "count": count,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        topic_arity = d.pop("topic_arity")

        topic_types = cast(list[str], d.pop("topic_types"))

        value_type = d.pop("value_type")

        count = d.pop("count")

        observed_shape = cls(
            topic_arity=topic_arity,
            topic_types=topic_types,
            value_type=value_type,
            count=count,
        )

        observed_shape.additional_properties = d
        return observed_shape

    @property
    def additional_keys(self) -> list[str]:
        return list(self.additional_properties.keys())

    def __getitem__(self, key: str) -> Any:
        return self.additional_properties[key]

    def __setitem__(self, key: str, value: Any) -> None:
        self.additional_properties[key] = value

    def __delitem__(self, key: str) -> None:
        del self.additional_properties[key]

    def __contains__(self, key: str) -> bool:
        return key in self.additional_properties

from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..models.post_api_v1_labels_body_scope import PostApiV1LabelsBodyScope

T = TypeVar("T", bound="PostApiV1LabelsBody")


@_attrs_define
class PostApiV1LabelsBody:
    """
    Attributes:
        label (str):
        value (str):
        scope (PostApiV1LabelsBodyScope):
    """

    label: str
    value: str
    scope: PostApiV1LabelsBodyScope
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        label = self.label

        value = self.value

        scope = self.scope.value

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "label": label,
                "value": value,
                "scope": scope,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        label = d.pop("label")

        value = d.pop("value")

        scope = PostApiV1LabelsBodyScope(d.pop("scope"))

        post_api_v1_labels_body = cls(
            label=label,
            value=value,
            scope=scope,
        )

        post_api_v1_labels_body.additional_properties = d
        return post_api_v1_labels_body

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

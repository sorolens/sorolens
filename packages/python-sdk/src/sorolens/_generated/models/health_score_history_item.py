from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

T = TypeVar("T", bound="HealthScoreHistoryItem")


@_attrs_define
class HealthScoreHistoryItem:
    """
    Attributes:
        computed_at (datetime.datetime):
        score (int):
    """

    computed_at: datetime.datetime
    score: int
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        computed_at = self.computed_at.isoformat()

        score = self.score

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "computed_at": computed_at,
                "score": score,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        computed_at = datetime.datetime.fromisoformat(d.pop("computed_at"))

        score = d.pop("score")

        health_score_history_item = cls(
            computed_at=computed_at,
            score=score,
        )

        health_score_history_item.additional_properties = d
        return health_score_history_item

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

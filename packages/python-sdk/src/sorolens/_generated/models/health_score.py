from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.health_score_components import HealthScoreComponents
    from ..models.health_score_history_item import HealthScoreHistoryItem


T = TypeVar("T", bound="HealthScore")


@_attrs_define
class HealthScore:
    """
    Attributes:
        contract_id (str):
        score (int):
        components (HealthScoreComponents):
        computed_at (datetime.datetime):
        history (list[HealthScoreHistoryItem] | Unset): Recorded hourly snapshots, oldest first. Included when
            history_limit is requested.
    """

    contract_id: str
    score: int
    components: HealthScoreComponents
    computed_at: datetime.datetime
    history: list[HealthScoreHistoryItem] | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        contract_id = self.contract_id

        score = self.score

        components = self.components.to_dict()

        computed_at = self.computed_at.isoformat()

        history: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.history, Unset):
            history = []
            for history_item_data in self.history:
                history_item = history_item_data.to_dict()
                history.append(history_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "contract_id": contract_id,
                "score": score,
                "components": components,
                "computed_at": computed_at,
            }
        )
        if history is not UNSET:
            field_dict["history"] = history

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.health_score_components import HealthScoreComponents
        from ..models.health_score_history_item import HealthScoreHistoryItem

        d = dict(src_dict)
        contract_id = d.pop("contract_id")

        score = d.pop("score")

        components = HealthScoreComponents.from_dict(d.pop("components"))

        computed_at = datetime.datetime.fromisoformat(d.pop("computed_at"))

        _history = d.pop("history", UNSET)
        history: list[HealthScoreHistoryItem] | Unset = UNSET
        if _history is not UNSET:
            history = []
            for history_item_data in _history:
                history_item = HealthScoreHistoryItem.from_dict(history_item_data)

                history.append(history_item)

        health_score = cls(
            contract_id=contract_id,
            score=score,
            components=components,
            computed_at=computed_at,
            history=history,
        )

        health_score.additional_properties = d
        return health_score

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

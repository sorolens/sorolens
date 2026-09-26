from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

if TYPE_CHECKING:
    from ..models.v2_contract_health_score_components import (
        V2ContractHealthScoreComponents,
    )


T = TypeVar("T", bound="V2ContractHealthScore")


@_attrs_define
class V2ContractHealthScore:
    """
    Attributes:
        contract_id (str):
        score (int):
        components (V2ContractHealthScoreComponents):
        computed_at (datetime.datetime):
    """

    contract_id: str
    score: int
    components: V2ContractHealthScoreComponents
    computed_at: datetime.datetime
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        contract_id = self.contract_id

        score = self.score

        components = self.components.to_dict()

        computed_at = self.computed_at.isoformat()

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

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.v2_contract_health_score_components import (
            V2ContractHealthScoreComponents,
        )

        d = dict(src_dict)
        contract_id = d.pop("contract_id")

        score = d.pop("score")

        components = V2ContractHealthScoreComponents.from_dict(d.pop("components"))

        computed_at = datetime.datetime.fromisoformat(d.pop("computed_at"))

        v2_contract_health_score = cls(
            contract_id=contract_id,
            score=score,
            components=components,
            computed_at=computed_at,
        )

        v2_contract_health_score.additional_properties = d
        return v2_contract_health_score

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

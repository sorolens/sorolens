from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.contract_stats import ContractStats
    from ..models.event import Event
    from ..models.health_score import HealthScore
    from ..models.invocation import Invocation


T = TypeVar("T", bound="ContractSummary")


@_attrs_define
class ContractSummary:
    """
    Attributes:
        contract_id (str):
        network (str):
        label (str):
        status (str):
        generated_at (datetime.datetime):
        stats (ContractStats):
        latest_event (Event | Unset):
        latest_invocation (Invocation | Unset):
        health_score (HealthScore | Unset):
    """

    contract_id: str
    network: str
    label: str
    status: str
    generated_at: datetime.datetime
    stats: ContractStats
    latest_event: Event | Unset = UNSET
    latest_invocation: Invocation | Unset = UNSET
    health_score: HealthScore | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        contract_id = self.contract_id

        network = self.network

        label = self.label

        status = self.status

        generated_at = self.generated_at.isoformat()

        stats = self.stats.to_dict()

        latest_event: dict[str, Any] | Unset = UNSET
        if not isinstance(self.latest_event, Unset):
            latest_event = self.latest_event.to_dict()

        latest_invocation: dict[str, Any] | Unset = UNSET
        if not isinstance(self.latest_invocation, Unset):
            latest_invocation = self.latest_invocation.to_dict()

        health_score: dict[str, Any] | Unset = UNSET
        if not isinstance(self.health_score, Unset):
            health_score = self.health_score.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "contract_id": contract_id,
                "network": network,
                "label": label,
                "status": status,
                "generated_at": generated_at,
                "stats": stats,
            }
        )
        if latest_event is not UNSET:
            field_dict["latest_event"] = latest_event
        if latest_invocation is not UNSET:
            field_dict["latest_invocation"] = latest_invocation
        if health_score is not UNSET:
            field_dict["health_score"] = health_score

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.contract_stats import ContractStats
        from ..models.event import Event
        from ..models.health_score import HealthScore
        from ..models.invocation import Invocation

        d = dict(src_dict)
        contract_id = d.pop("contract_id")

        network = d.pop("network")

        label = d.pop("label")

        status = d.pop("status")

        generated_at = datetime.datetime.fromisoformat(d.pop("generated_at"))

        stats = ContractStats.from_dict(d.pop("stats"))

        _latest_event = d.pop("latest_event", UNSET)
        latest_event: Event | Unset
        if isinstance(_latest_event, Unset):
            latest_event = UNSET
        else:
            latest_event = Event.from_dict(_latest_event)

        _latest_invocation = d.pop("latest_invocation", UNSET)
        latest_invocation: Invocation | Unset
        if isinstance(_latest_invocation, Unset):
            latest_invocation = UNSET
        else:
            latest_invocation = Invocation.from_dict(_latest_invocation)

        _health_score = d.pop("health_score", UNSET)
        health_score: HealthScore | Unset
        if isinstance(_health_score, Unset):
            health_score = UNSET
        else:
            health_score = HealthScore.from_dict(_health_score)

        contract_summary = cls(
            contract_id=contract_id,
            network=network,
            label=label,
            status=status,
            generated_at=generated_at,
            stats=stats,
            latest_event=latest_event,
            latest_invocation=latest_invocation,
            health_score=health_score,
        )

        contract_summary.additional_properties = d
        return contract_summary

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

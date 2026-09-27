from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, TypeVar, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

T = TypeVar("T", bound="GroupContract")


@_attrs_define
class GroupContract:
    """
    Attributes:
        contract_id (str):
        network (str):
        label (str):
        status (str):
        health_score (int | None):
        last_activity_at (datetime.datetime | None):
    """

    contract_id: str
    network: str
    label: str
    status: str
    health_score: int | None
    last_activity_at: datetime.datetime | None
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        contract_id = self.contract_id

        network = self.network

        label = self.label

        status = self.status

        health_score: int | None
        health_score = self.health_score

        last_activity_at: None | str
        if isinstance(self.last_activity_at, datetime.datetime):
            last_activity_at = self.last_activity_at.isoformat()
        else:
            last_activity_at = self.last_activity_at

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "contract_id": contract_id,
                "network": network,
                "label": label,
                "status": status,
                "health_score": health_score,
                "last_activity_at": last_activity_at,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        contract_id = d.pop("contract_id")

        network = d.pop("network")

        label = d.pop("label")

        status = d.pop("status")

        def _parse_health_score(data: object) -> int | None:
            if data is None:
                return data
            return cast(int | None, data)

        health_score = _parse_health_score(d.pop("health_score"))

        def _parse_last_activity_at(data: object) -> datetime.datetime | None:
            if data is None:
                return data
            try:
                if not isinstance(data, str):
                    raise TypeError()
                last_activity_at_type_0 = datetime.datetime.fromisoformat(data)

                return last_activity_at_type_0
            except (TypeError, ValueError, AttributeError, KeyError):
                pass
            return cast(datetime.datetime | None, data)

        last_activity_at = _parse_last_activity_at(d.pop("last_activity_at"))

        group_contract = cls(
            contract_id=contract_id,
            network=network,
            label=label,
            status=status,
            health_score=health_score,
            last_activity_at=last_activity_at,
        )

        group_contract.additional_properties = d
        return group_contract

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

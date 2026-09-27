from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

if TYPE_CHECKING:
    from ..models.contract_event_rate import ContractEventRate


T = TypeVar("T", bound="LiveActivityResponse200")


@_attrs_define
class LiveActivityResponse200:
    """
    Attributes:
        minutes (int):
        window_start (datetime.datetime):
        contracts (list[ContractEventRate]):
    """

    minutes: int
    window_start: datetime.datetime
    contracts: list[ContractEventRate]
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        minutes = self.minutes

        window_start = self.window_start.isoformat()

        contracts = []
        for contracts_item_data in self.contracts:
            contracts_item = contracts_item_data.to_dict()
            contracts.append(contracts_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "minutes": minutes,
                "window_start": window_start,
                "contracts": contracts,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.contract_event_rate import ContractEventRate

        d = dict(src_dict)
        minutes = d.pop("minutes")

        window_start = datetime.datetime.fromisoformat(d.pop("window_start"))

        contracts = []
        _contracts = d.pop("contracts")
        for contracts_item_data in _contracts:
            contracts_item = ContractEventRate.from_dict(contracts_item_data)

            contracts.append(contracts_item)

        live_activity_response_200 = cls(
            minutes=minutes,
            window_start=window_start,
            contracts=contracts,
        )

        live_activity_response_200.additional_properties = d
        return live_activity_response_200

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

from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

if TYPE_CHECKING:
    from ..models.contract_alert import ContractAlert


T = TypeVar("T", bound="ListContractAlertsResponse200")


@_attrs_define
class ListContractAlertsResponse200:
    """
    Attributes:
        alerts (list[ContractAlert]):
        next_cursor (str): Cursor for the next (older) page; empty when no more pages.
    """

    alerts: list[ContractAlert]
    next_cursor: str
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        alerts = []
        for alerts_item_data in self.alerts:
            alerts_item = alerts_item_data.to_dict()
            alerts.append(alerts_item)

        next_cursor = self.next_cursor

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "alerts": alerts,
                "next_cursor": next_cursor,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.contract_alert import ContractAlert

        d = dict(src_dict)
        alerts = []
        _alerts = d.pop("alerts")
        for alerts_item_data in _alerts:
            alerts_item = ContractAlert.from_dict(alerts_item_data)

            alerts.append(alerts_item)

        next_cursor = d.pop("next_cursor")

        list_contract_alerts_response_200 = cls(
            alerts=alerts,
            next_cursor=next_cursor,
        )

        list_contract_alerts_response_200.additional_properties = d
        return list_contract_alerts_response_200

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

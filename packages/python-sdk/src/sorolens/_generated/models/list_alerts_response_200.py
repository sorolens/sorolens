from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.alert_group import AlertGroup
    from ..models.contract_alert import ContractAlert


T = TypeVar("T", bound="ListAlertsResponse200")


@_attrs_define
class ListAlertsResponse200:
    """
    Attributes:
        next_cursor (str):
        groups (list[AlertGroup] | Unset): Present when flat is not set.
        alerts (list[ContractAlert] | Unset): Present when flat=true.
    """

    next_cursor: str
    groups: list[AlertGroup] | Unset = UNSET
    alerts: list[ContractAlert] | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        next_cursor = self.next_cursor

        groups: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.groups, Unset):
            groups = []
            for groups_item_data in self.groups:
                groups_item = groups_item_data.to_dict()
                groups.append(groups_item)

        alerts: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.alerts, Unset):
            alerts = []
            for alerts_item_data in self.alerts:
                alerts_item = alerts_item_data.to_dict()
                alerts.append(alerts_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "next_cursor": next_cursor,
            }
        )
        if groups is not UNSET:
            field_dict["groups"] = groups
        if alerts is not UNSET:
            field_dict["alerts"] = alerts

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.alert_group import AlertGroup
        from ..models.contract_alert import ContractAlert

        d = dict(src_dict)
        next_cursor = d.pop("next_cursor")

        _groups = d.pop("groups", UNSET)
        groups: list[AlertGroup] | Unset = UNSET
        if _groups is not UNSET:
            groups = []
            for groups_item_data in _groups:
                groups_item = AlertGroup.from_dict(groups_item_data)

                groups.append(groups_item)

        _alerts = d.pop("alerts", UNSET)
        alerts: list[ContractAlert] | Unset = UNSET
        if _alerts is not UNSET:
            alerts = []
            for alerts_item_data in _alerts:
                alerts_item = ContractAlert.from_dict(alerts_item_data)

                alerts.append(alerts_item)

        list_alerts_response_200 = cls(
            next_cursor=next_cursor,
            groups=groups,
            alerts=alerts,
        )

        list_alerts_response_200.additional_properties = d
        return list_alerts_response_200

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

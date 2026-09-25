from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from typing import cast

if TYPE_CHECKING:
  from ..models.contract_alert import ContractAlert





T = TypeVar("T", bound="ListWatchdogAlertsResponse200")



@_attrs_define
class ListWatchdogAlertsResponse200:
    """ 
        Attributes:
            alerts (list[ContractAlert]):
     """

    alerts: list[ContractAlert]
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)





    def to_dict(self) -> dict[str, Any]:
        from ..models.contract_alert import ContractAlert # noqa: PLC0415
        alerts = []
        for alerts_item_data in self.alerts:
            alerts_item = alerts_item_data.to_dict()
            alerts.append(alerts_item)




        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({
            "alerts": alerts,
        })

        return field_dict



    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        from ..models.contract_alert import ContractAlert # noqa: PLC0415
        d = dict(src_dict)
        alerts = []
        _alerts = d.pop("alerts")
        for alerts_item_data in (_alerts):
            alerts_item = ContractAlert.from_dict(alerts_item_data)



            alerts.append(alerts_item)


        list_watchdog_alerts_response_200 = cls(
            alerts=alerts,
        )


        list_watchdog_alerts_response_200.additional_properties = d
        return list_watchdog_alerts_response_200

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

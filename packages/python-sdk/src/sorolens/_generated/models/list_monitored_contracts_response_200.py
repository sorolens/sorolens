from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from typing import cast

if TYPE_CHECKING:
  from ..models.monitored_contract import MonitoredContract





T = TypeVar("T", bound="ListMonitoredContractsResponse200")



@_attrs_define
class ListMonitoredContractsResponse200:
    """ 
        Attributes:
            contracts (list[MonitoredContract]):
            next_cursor (str):
     """

    contracts: list[MonitoredContract]
    next_cursor: str
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)





    def to_dict(self) -> dict[str, Any]:
        from ..models.monitored_contract import MonitoredContract # noqa: PLC0415
        contracts = []
        for contracts_item_data in self.contracts:
            contracts_item = contracts_item_data.to_dict()
            contracts.append(contracts_item)



        next_cursor = self.next_cursor


        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({
            "contracts": contracts,
            "next_cursor": next_cursor,
        })

        return field_dict



    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        from ..models.monitored_contract import MonitoredContract # noqa: PLC0415
        d = dict(src_dict)
        contracts = []
        _contracts = d.pop("contracts")
        for contracts_item_data in (_contracts):
            contracts_item = MonitoredContract.from_dict(contracts_item_data)



            contracts.append(contracts_item)


        next_cursor = d.pop("next_cursor")

        list_monitored_contracts_response_200 = cls(
            contracts=contracts,
            next_cursor=next_cursor,
        )


        list_monitored_contracts_response_200.additional_properties = d
        return list_monitored_contracts_response_200

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

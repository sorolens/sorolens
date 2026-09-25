from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..models.register_contract_body_network import RegisterContractBodyNetwork
from ..types import UNSET, Unset






T = TypeVar("T", bound="RegisterContractBody")



@_attrs_define
class RegisterContractBody:
    """ 
        Attributes:
            id (str): Stellar contract ID (56 chars, starts with C).
            network (RegisterContractBodyNetwork):
            label (str | Unset): Optional human-friendly label.
     """

    id: str
    network: RegisterContractBodyNetwork
    label: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)





    def to_dict(self) -> dict[str, Any]:
        id = self.id

        network = self.network.value

        label = self.label


        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({
            "id": id,
            "network": network,
        })
        if label is not UNSET:
            field_dict["label"] = label

        return field_dict



    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        id = d.pop("id")

        network = RegisterContractBodyNetwork(d.pop("network"))




        label = d.pop("label", UNSET)

        register_contract_body = cls(
            id=id,
            network=network,
            label=label,
        )


        register_contract_body.additional_properties = d
        return register_contract_body

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

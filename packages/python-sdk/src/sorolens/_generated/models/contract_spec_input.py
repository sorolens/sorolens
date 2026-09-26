from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.contract_spec_type import ContractSpecType


T = TypeVar("T", bound="ContractSpecInput")


@_attrs_define
class ContractSpecInput:
    """
    Attributes:
        name (str):
        type_ (ContractSpecType):
        doc (str | Unset):
    """

    name: str
    type_: ContractSpecType
    doc: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        type_ = self.type_.to_dict()

        doc = self.doc

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "name": name,
                "type": type_,
            }
        )
        if doc is not UNSET:
            field_dict["doc"] = doc

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.contract_spec_type import ContractSpecType

        d = dict(src_dict)
        name = d.pop("name")

        type_ = ContractSpecType.from_dict(d.pop("type"))

        doc = d.pop("doc", UNSET)

        contract_spec_input = cls(
            name=name,
            type_=type_,
            doc=doc,
        )

        contract_spec_input.additional_properties = d
        return contract_spec_input

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

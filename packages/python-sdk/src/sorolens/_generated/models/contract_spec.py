from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..types import UNSET, Unset

from ..types import UNSET, Unset
from typing import cast
import datetime

if TYPE_CHECKING:
  from ..models.contract_spec_tree import ContractSpecTree





T = TypeVar("T", bound="ContractSpec")



@_attrs_define
class ContractSpec:
    """ 
        Attributes:
            contract_id (str):
            parsed_at (datetime.datetime):
            spec (ContractSpecTree):
            wasm_hash (str | Unset):
     """

    contract_id: str
    parsed_at: datetime.datetime
    spec: ContractSpecTree
    wasm_hash: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)





    def to_dict(self) -> dict[str, Any]:
        from ..models.contract_spec_tree import ContractSpecTree # noqa: PLC0415
        contract_id = self.contract_id

        parsed_at = self.parsed_at.isoformat()

        spec = self.spec.to_dict()

        wasm_hash = self.wasm_hash


        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({
            "contract_id": contract_id,
            "parsed_at": parsed_at,
            "spec": spec,
        })
        if wasm_hash is not UNSET:
            field_dict["wasm_hash"] = wasm_hash

        return field_dict



    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.contract_spec_tree import ContractSpecTree # noqa: PLC0415
        d = dict(src_dict)
        contract_id = d.pop("contract_id")

        parsed_at = datetime.datetime.fromisoformat(d.pop("parsed_at"))




        spec = ContractSpecTree.from_dict(d.pop("spec"))




        wasm_hash = d.pop("wasm_hash", UNSET)

        contract_spec = cls(
            contract_id=contract_id,
            parsed_at=parsed_at,
            spec=spec,
            wasm_hash=wasm_hash,
        )


        contract_spec.additional_properties = d
        return contract_spec

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

from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..types import UNSET, Unset

T = TypeVar("T", bound="ContractSpecType")


@_attrs_define
class ContractSpecType:
    """
    Attributes:
        kind (str):
        name (str | Unset):
        elem (ContractSpecType | Unset):
        key (ContractSpecType | Unset):
        value (ContractSpecType | Unset):
        ok (ContractSpecType | Unset):
        err (ContractSpecType | Unset):
        tuple_ (list[ContractSpecType] | Unset):
        n (int | Unset):
    """

    kind: str
    name: str | Unset = UNSET
    elem: ContractSpecType | Unset = UNSET
    key: ContractSpecType | Unset = UNSET
    value: ContractSpecType | Unset = UNSET
    ok: ContractSpecType | Unset = UNSET
    err: ContractSpecType | Unset = UNSET
    tuple_: list[ContractSpecType] | Unset = UNSET
    n: int | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        kind = self.kind

        name = self.name

        elem: dict[str, Any] | Unset = UNSET
        if not isinstance(self.elem, Unset):
            elem = self.elem.to_dict()

        key: dict[str, Any] | Unset = UNSET
        if not isinstance(self.key, Unset):
            key = self.key.to_dict()

        value: dict[str, Any] | Unset = UNSET
        if not isinstance(self.value, Unset):
            value = self.value.to_dict()

        ok: dict[str, Any] | Unset = UNSET
        if not isinstance(self.ok, Unset):
            ok = self.ok.to_dict()

        err: dict[str, Any] | Unset = UNSET
        if not isinstance(self.err, Unset):
            err = self.err.to_dict()

        tuple_: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.tuple_, Unset):
            tuple_ = []
            for tuple_item_data in self.tuple_:
                tuple_item = tuple_item_data.to_dict()
                tuple_.append(tuple_item)

        n = self.n

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "kind": kind,
            }
        )
        if name is not UNSET:
            field_dict["name"] = name
        if elem is not UNSET:
            field_dict["elem"] = elem
        if key is not UNSET:
            field_dict["key"] = key
        if value is not UNSET:
            field_dict["value"] = value
        if ok is not UNSET:
            field_dict["ok"] = ok
        if err is not UNSET:
            field_dict["err"] = err
        if tuple_ is not UNSET:
            field_dict["tuple"] = tuple_
        if n is not UNSET:
            field_dict["n"] = n

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        kind = d.pop("kind")

        name = d.pop("name", UNSET)

        _elem = d.pop("elem", UNSET)
        elem: ContractSpecType | Unset
        if isinstance(_elem, Unset):
            elem = UNSET
        else:
            elem = ContractSpecType.from_dict(_elem)

        _key = d.pop("key", UNSET)
        key: ContractSpecType | Unset
        if isinstance(_key, Unset):
            key = UNSET
        else:
            key = ContractSpecType.from_dict(_key)

        _value = d.pop("value", UNSET)
        value: ContractSpecType | Unset
        if isinstance(_value, Unset):
            value = UNSET
        else:
            value = ContractSpecType.from_dict(_value)

        _ok = d.pop("ok", UNSET)
        ok: ContractSpecType | Unset
        if isinstance(_ok, Unset):
            ok = UNSET
        else:
            ok = ContractSpecType.from_dict(_ok)

        _err = d.pop("err", UNSET)
        err: ContractSpecType | Unset
        if isinstance(_err, Unset):
            err = UNSET
        else:
            err = ContractSpecType.from_dict(_err)

        _tuple_ = d.pop("tuple", UNSET)
        tuple_: list[ContractSpecType] | Unset = UNSET
        if _tuple_ is not UNSET:
            tuple_ = []
            for tuple_item_data in _tuple_:
                tuple_item = ContractSpecType.from_dict(tuple_item_data)

                tuple_.append(tuple_item)

        n = d.pop("n", UNSET)

        contract_spec_type = cls(
            kind=kind,
            name=name,
            elem=elem,
            key=key,
            value=value,
            ok=ok,
            err=err,
            tuple_=tuple_,
            n=n,
        )

        contract_spec_type.additional_properties = d
        return contract_spec_type

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

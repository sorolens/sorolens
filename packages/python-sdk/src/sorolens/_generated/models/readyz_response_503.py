from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..types import UNSET, Unset
from typing import cast

if TYPE_CHECKING:
  from ..models.readyz_response_503_checks import ReadyzResponse503Checks





T = TypeVar("T", bound="ReadyzResponse503")



@_attrs_define
class ReadyzResponse503:
    """ 
        Attributes:
            status (str | Unset):  Example: unavailable.
            checks (ReadyzResponse503Checks | Unset):
     """

    status: str | Unset = UNSET
    checks: ReadyzResponse503Checks | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)





    def to_dict(self) -> dict[str, Any]:
        from ..models.readyz_response_503_checks import ReadyzResponse503Checks # noqa: PLC0415
        status = self.status

        checks: dict[str, Any] | Unset = UNSET
        if not isinstance(self.checks, Unset):
            checks = self.checks.to_dict()


        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({
        })
        if status is not UNSET:
            field_dict["status"] = status
        if checks is not UNSET:
            field_dict["checks"] = checks

        return field_dict



    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        from ..models.readyz_response_503_checks import ReadyzResponse503Checks # noqa: PLC0415
        d = dict(src_dict)
        status = d.pop("status", UNSET)

        _checks = d.pop("checks", UNSET)
        checks: ReadyzResponse503Checks | Unset
        if isinstance(_checks,  Unset):
            checks = UNSET
        else:
            checks = ReadyzResponse503Checks.from_dict(_checks)




        readyz_response_503 = cls(
            status=status,
            checks=checks,
        )


        readyz_response_503.additional_properties = d
        return readyz_response_503

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

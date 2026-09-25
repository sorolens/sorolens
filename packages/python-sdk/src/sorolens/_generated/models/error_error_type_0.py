from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..models.error_error_type_0_code import ErrorErrorType0Code
from ..types import UNSET, Unset






T = TypeVar("T", bound="ErrorErrorType0")



@_attrs_define
class ErrorErrorType0:
    """ 
        Attributes:
            code (ErrorErrorType0Code):
            message (str):
            request_id (str | Unset):
     """

    code: ErrorErrorType0Code
    message: str
    request_id: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)





    def to_dict(self) -> dict[str, Any]:
        code = self.code.value

        message = self.message

        request_id = self.request_id


        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({
            "code": code,
            "message": message,
        })
        if request_id is not UNSET:
            field_dict["request_id"] = request_id

        return field_dict



    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        code = ErrorErrorType0Code(d.pop("code"))




        message = d.pop("message")

        request_id = d.pop("request_id", UNSET)

        error_error_type_0 = cls(
            code=code,
            message=message,
            request_id=request_id,
        )


        error_error_type_0.additional_properties = d
        return error_error_type_0

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

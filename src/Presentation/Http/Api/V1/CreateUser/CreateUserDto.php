<?php

declare(strict_types=1);
declare(ticks=1000);

namespace App\Presentation\Http\Api\V1\CreateUser;

use Symfony\Component\Serializer\Annotation\SerializedName;

final readonly class CreateUserDto
{
    public function __construct(
        #[SerializedName('first_name')]
        public string $firstName,

        #[SerializedName('last_name')]
        public string $lastName,

        #[SerializedName('email')]
        public string $email,
    )
    {
    }
}

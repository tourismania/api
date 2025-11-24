<?php

declare(strict_types=1);
declare(ticks=1000);

namespace App\Application\UseCases\GetMe;

use App\Domain\Repository\UserRepositoryInterface;
use App\Domain\ValueObject\RightsDescribe;
use Symfony\Component\Messenger\Attribute\AsMessageHandler;

readonly class GetMeResult
{
    /**
     * @param int $id
     * @param string $email
     * @param string $phone
     * @param string $firstName
     * @param string $lastName
     * @param RightsDescribe $rights
     */
    public function __construct(
        public int            $id,
        public string         $email,
        public string         $phone,
        public string         $firstName,
        public string         $lastName,
        public RightsDescribe $rights,
    ){}

}
